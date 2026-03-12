package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	policyv1alpha1 "acs-next.stackrox.io/apis/policy.stackrox.io/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"acs-next-runtime-evaluator/internal/subscriber"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine := subscriber.NewEngine()

	// Set up K8s CRD informers
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Warning: not running in-cluster, loading default policies: %v", err)
		engine.LoadDefaultPolicies()
	} else {
		dynClient, err := dynamic.NewForConfig(restCfg)
		if err != nil {
			log.Fatalf("Failed to create dynamic client: %v", err)
		}
		if !startCRDInformers(ctx, dynClient, engine) {
			log.Println("CRDs not available, loading default policies")
			engine.LoadDefaultPolicies()
		}
	}

	// Connect to NATS and start subscribing
	natsURL := getEnv("NATS_URL", "nats://acs-broker.acs-next.svc:4222")
	subCfg := subscriber.Config{
		NATSURL: natsURL,
		TLSCert: getEnv("TLS_CERT", ""),
		TLSKey:  getEnv("TLS_KEY", ""),
		TLSCA:   getEnv("TLS_CA", ""),
	}

	sub, err := subscriber.New(subCfg, engine)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}

	if err := sub.Start(ctx); err != nil {
		log.Fatalf("Failed to start NATS subscriber: %v", err)
	}
	log.Printf("Runtime evaluator connected to %s", natsURL)

	// Wait for shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	log.Printf("Received signal %v, shutting down...", sig)
	cancel()
	sub.Shutdown()
	log.Println("Runtime evaluator shutdown complete")
}

var (
	clusterPolicyGVR = schema.GroupVersionResource{
		Group:    "policy.stackrox.io",
		Version:  "v1alpha1",
		Resource: "clusterstackroxpolicies",
	}
	namespacedPolicyGVR = schema.GroupVersionResource{
		Group:    "policy.stackrox.io",
		Version:  "v1alpha1",
		Resource: "stackroxpolicies",
	}
)

func startCRDInformers(ctx context.Context, dynClient dynamic.Interface, engine *subscriber.Engine) bool {
	_, err := dynClient.Resource(clusterPolicyGVR).List(ctx, subscriber.ListOptions())
	if err != nil {
		log.Printf("ClusterStackroxPolicy CRD not available: %v", err)
		return false
	}

	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, 30*time.Second)

	clusterInformer := factory.ForResource(clusterPolicyGVR).Informer()
	clusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			p, err := convertToClusterPolicy(obj)
			if err != nil {
				log.Printf("Failed to convert cluster policy: %v", err)
				return
			}
			engine.SetClusterPolicy(p)
		},
		UpdateFunc: func(_, newObj interface{}) {
			p, err := convertToClusterPolicy(newObj)
			if err != nil {
				log.Printf("Failed to convert cluster policy: %v", err)
				return
			}
			engine.SetClusterPolicy(p)
		},
		DeleteFunc: func(obj interface{}) {
			p, err := convertToClusterPolicy(obj)
			if err != nil {
				return
			}
			engine.DeleteClusterPolicy(p.Name)
		},
	})

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	log.Printf("CRD informers synced: %d cluster policies", engine.PolicyCount())
	return true
}

func convertToClusterPolicy(obj interface{}) (*policyv1alpha1.ClusterStackroxPolicy, error) {
	unstructured, ok := obj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("object is not unstructured")
	}

	p := &policyv1alpha1.ClusterStackroxPolicy{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), p)
	if err != nil {
		return nil, fmt.Errorf("convert to ClusterStackroxPolicy: %w", err)
	}
	return p, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
