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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"acs-next-admission-controller/internal/policy"
	"acs-next-admission-controller/internal/subscriber"
	"acs-next-admission-controller/internal/webhook"
)

func main() {
	engine := policy.NewEngine()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Try to set up K8s clients and CRD informers
	var k8sClient kubernetes.Interface
	crdAvailable := false

	if restCfg, err := rest.InClusterConfig(); err == nil {
		k8sClient, err = kubernetes.NewForConfig(restCfg)
		if err != nil {
			log.Printf("Warning: failed to create K8s client: %v", err)
		}

		// Set up dynamic client for CRD watching
		dynClient, err := dynamic.NewForConfig(restCfg)
		if err != nil {
			log.Printf("Warning: failed to create dynamic client: %v", err)
		} else {
			statusUpdater := &crdStatusUpdater{dynClient: dynClient}
			engine.SetStatusUpdater(statusUpdater)
			crdAvailable = startCRDInformers(ctx, dynClient, engine)
		}
	} else {
		log.Printf("Warning: not running in-cluster, K8s client unavailable: %v", err)
	}

	// Fall back to default policies if CRDs aren't available
	if !crdAvailable {
		log.Println("CRDs not available, loading default policies")
		engine.LoadDefaultPolicies()
	}

	// Connect to NATS for alert publishing and runtime events
	natsURL := getEnv("NATS_URL", "nats://acs-broker.acs-next.svc:4222")
	subCfg := subscriber.Config{
		NATSURL: natsURL,
		TLSCert: getEnv("TLS_CERT", ""),
		TLSKey:  getEnv("TLS_KEY", ""),
		TLSCA:   getEnv("TLS_CA", ""),
	}

	var alertPub policy.AlertPublisher
	sub, err := subscriber.New(subCfg, engine, k8sClient)
	if err != nil {
		log.Printf("Warning: failed to connect to NATS, runtime enforcement disabled: %v", err)
	} else {
		alertPub = sub.AlertPublisher()
		if err := sub.Start(ctx); err != nil {
			log.Printf("Warning: failed to start NATS subscriber: %v", err)
		} else {
			log.Printf("NATS subscriber connected to %s", natsURL)
		}
	}

	// Start webhook server
	webhookCfg := webhook.Config{
		WebhookPort: getEnvInt("WEBHOOK_PORT", 8443),
		HealthPort:  getEnvInt("HEALTH_PORT", 8080),
		TLSCert:     getEnv("WEBHOOK_TLS_CERT", "/certs/webhook/tls.crt"),
		TLSKey:      getEnv("WEBHOOK_TLS_KEY", "/certs/webhook/tls.key"),
	}

	webhookSrv, err := webhook.New(webhookCfg, engine, alertPub)
	if err != nil {
		log.Fatalf("Failed to create webhook server: %v", err)
	}

	if err := webhookSrv.Start(); err != nil {
		log.Fatalf("Failed to start webhook server: %v", err)
	}
	log.Printf("Webhook server started on port %d", webhookCfg.WebhookPort)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	log.Printf("Received signal %v, shutting down...", sig)
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	webhookSrv.Shutdown(shutdownCtx)

	if sub != nil {
		sub.Shutdown()
	}
	log.Println("Admission controller shutdown complete")
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

func startCRDInformers(ctx context.Context, dynClient dynamic.Interface, engine *policy.Engine) bool {
	// Check if the CRD exists by trying a list
	_, err := dynClient.Resource(clusterPolicyGVR).List(ctx, policy.ListOptions())
	if err != nil {
		log.Printf("ClusterStackroxPolicy CRD not available: %v", err)
		return false
	}

	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, 30*time.Second)

	// ClusterStackroxPolicy informer
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
				log.Printf("Failed to convert deleted cluster policy: %v", err)
				return
			}
			engine.DeleteClusterPolicy(p.Name)
		},
	})

	// StackroxPolicy informer
	nsInformer := factory.ForResource(namespacedPolicyGVR).Informer()
	nsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			p, err := convertToNamespacedPolicy(obj)
			if err != nil {
				log.Printf("Failed to convert namespaced policy: %v", err)
				return
			}
			engine.SetNamespacedPolicy(p)
		},
		UpdateFunc: func(_, newObj interface{}) {
			p, err := convertToNamespacedPolicy(newObj)
			if err != nil {
				log.Printf("Failed to convert namespaced policy: %v", err)
				return
			}
			engine.SetNamespacedPolicy(p)
		},
		DeleteFunc: func(obj interface{}) {
			p, err := convertToNamespacedPolicy(obj)
			if err != nil {
				log.Printf("Failed to convert deleted namespaced policy: %v", err)
				return
			}
			engine.DeleteNamespacedPolicy(p.Namespace, p.Name)
		},
	})

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	cluster, ns := engine.PolicyCount()
	log.Printf("CRD informers synced: %d cluster policies, %d namespaced policies", cluster, ns)
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

func convertToNamespacedPolicy(obj interface{}) (*policyv1alpha1.StackroxPolicy, error) {
	unstructured, ok := obj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("object is not unstructured")
	}

	p := &policyv1alpha1.StackroxPolicy{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), p)
	if err != nil {
		return nil, fmt.Errorf("convert to StackroxPolicy: %w", err)
	}
	return p, nil
}

// crdStatusUpdater implements policy.StatusUpdater using the dynamic client.
type crdStatusUpdater struct {
	dynClient dynamic.Interface
}

func (u *crdStatusUpdater) UpdateClusterPolicyStatus(ctx context.Context, name string, status policyv1alpha1.ClusterStackroxPolicyStatus) error {
	existing, err := u.dynClient.Resource(clusterPolicyGVR).Get(ctx, name, policy.GetOptions())
	if err != nil {
		return fmt.Errorf("get cluster policy %s: %w", name, err)
	}

	statusMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&status)
	if err != nil {
		return fmt.Errorf("convert status: %w", err)
	}

	existing.Object["status"] = statusMap
	_, err = u.dynClient.Resource(clusterPolicyGVR).UpdateStatus(ctx, existing, policy.UpdateOptions())
	return err
}

func (u *crdStatusUpdater) UpdateNamespacedPolicyStatus(ctx context.Context, namespace, name string, status policyv1alpha1.StackroxPolicyStatus) error {
	existing, err := u.dynClient.Resource(namespacedPolicyGVR).Namespace(namespace).Get(ctx, name, policy.GetOptions())
	if err != nil {
		return fmt.Errorf("get namespaced policy %s/%s: %w", namespace, name, err)
	}

	statusMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&status)
	if err != nil {
		return fmt.Errorf("convert status: %w", err)
	}

	existing.Object["status"] = statusMap
	_, err = u.dynClient.Resource(namespacedPolicyGVR).Namespace(namespace).UpdateStatus(ctx, existing, policy.UpdateOptions())
	return err
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	s := os.Getenv(key)
	if s == "" {
		return defaultValue
	}
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return defaultValue
	}
	return v
}
