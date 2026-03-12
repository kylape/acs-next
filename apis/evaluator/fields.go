package evaluator

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// FieldHandler evaluates a single fieldName against a pod spec or runtime event.
// Returns true if the field check matches (i.e., the condition is found).
type FieldHandler func(ctx *EvalContext, values []string) bool

// EvalContext carries the data being evaluated.
type EvalContext struct {
	PodSpec   *corev1.PodSpec
	Namespace string

	// Runtime event fields
	ProcessName string
	Executable  string
	DstPort     int
	EventType   string
}

var fieldHandlers = map[string]FieldHandler{}

// RegisterField registers a field handler for a given fieldName.
func RegisterField(fieldName string, handler FieldHandler) {
	fieldHandlers[fieldName] = handler
}

// GetFieldHandler returns the handler for a fieldName, or nil if not registered.
func GetFieldHandler(fieldName string) FieldHandler {
	return fieldHandlers[fieldName]
}

func init() {
	RegisterField("Privileged Container", fieldPrivilegedContainer)
	RegisterField("Image Tag", fieldImageTag)
	RegisterField("Container CPU Limit", fieldContainerCPULimit)
	RegisterField("Container Memory Limit", fieldContainerMemoryLimit)
	RegisterField("Host Network", fieldHostNetwork)
	RegisterField("Process Name", fieldProcessName)
	RegisterField("Port", fieldPort)
}

func fieldPrivilegedContainer(ctx *EvalContext, _ []string) bool {
	if ctx.PodSpec == nil {
		return false
	}
	for _, c := range allContainers(ctx.PodSpec) {
		if c.SecurityContext != nil && c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
			return true
		}
	}
	return false
}

func fieldImageTag(ctx *EvalContext, values []string) bool {
	if ctx.PodSpec == nil {
		return false
	}
	for _, c := range allContainers(ctx.PodSpec) {
		image := c.Image
		if image == "" {
			continue
		}
		if strings.Contains(image, "@") {
			continue // digest-pinned
		}
		parts := strings.Split(image, ":")
		tag := ""
		if len(parts) > 1 {
			tag = parts[len(parts)-1]
		}
		// If values are specified, check if the tag matches any value
		if len(values) > 0 {
			for _, v := range values {
				if v == tag || (v == "latest" && tag == "") {
					return true
				}
			}
		} else {
			// No values = check for latest/untagged
			if tag == "" || tag == "latest" {
				return true
			}
		}
	}
	return false
}

func fieldContainerCPULimit(ctx *EvalContext, _ []string) bool {
	if ctx.PodSpec == nil {
		return false
	}
	for _, c := range allContainers(ctx.PodSpec) {
		if c.Resources.Limits == nil {
			return true // missing = violation
		}
		if _, ok := c.Resources.Limits[corev1.ResourceCPU]; !ok {
			return true
		}
	}
	return false
}

func fieldContainerMemoryLimit(ctx *EvalContext, _ []string) bool {
	if ctx.PodSpec == nil {
		return false
	}
	for _, c := range allContainers(ctx.PodSpec) {
		if c.Resources.Limits == nil {
			return true
		}
		if _, ok := c.Resources.Limits[corev1.ResourceMemory]; !ok {
			return true
		}
	}
	return false
}

func fieldHostNetwork(ctx *EvalContext, _ []string) bool {
	if ctx.PodSpec == nil {
		return false
	}
	return ctx.PodSpec.HostNetwork
}

func fieldProcessName(ctx *EvalContext, values []string) bool {
	if ctx.Executable == "" {
		return false
	}
	for _, v := range values {
		if v == ctx.Executable || v == ctx.ProcessName {
			return true
		}
	}
	return false
}

func fieldPort(ctx *EvalContext, values []string) bool {
	if ctx.DstPort == 0 {
		return false
	}
	for _, v := range values {
		if v == fmt.Sprintf("%d", ctx.DstPort) {
			return true
		}
	}
	return false
}

func allContainers(podSpec *corev1.PodSpec) []corev1.Container {
	all := make([]corev1.Container, 0, len(podSpec.InitContainers)+len(podSpec.Containers))
	all = append(all, podSpec.InitContainers...)
	all = append(all, podSpec.Containers...)
	return all
}
