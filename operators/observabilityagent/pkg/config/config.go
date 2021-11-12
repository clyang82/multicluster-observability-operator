// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package config

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	mcoshared "github.com/open-cluster-management/multicluster-observability-operator/operators/multiclusterobservability/api/shared"
)

const (
	AllowlistCustomConfigMapName = "observability-metrics-custom-allowlist"

	EndpointControllerImgName = "endpoint-monitoring-operator"
	EndpointControllerKey     = "endpoint_monitoring_operator"

	AlertmanagerAccessorSAName     = "observability-alertmanager-accessor"
	AlertmanagerAccessorSecretName = "observability-alertmanager-accessor"

	MetricsCollectorCPURequets    = "10m"
	MetricsCollectorMemoryRequets = "100Mi"
	MetricsCollectorCPULimits     = ""
	MetricsCollectorMemoryLimits  = ""
)

func GetOBAResources(oba *mcoshared.ObservabilityAddonSpec) *corev1.ResourceRequirements {
	cpuRequests := MetricsCollectorCPURequets
	cpuLimits := MetricsCollectorCPULimits
	memoryRequests := MetricsCollectorMemoryRequets
	memoryLimits := MetricsCollectorMemoryLimits

	if oba.Resources != nil {
		if len(oba.Resources.Requests) != 0 {
			if oba.Resources.Requests.Cpu().String() != "0" {
				cpuRequests = oba.Resources.Requests.Cpu().String()
			}
			if oba.Resources.Requests.Memory().String() != "0" {
				memoryRequests = oba.Resources.Requests.Memory().String()
			}
		}
		if len(oba.Resources.Limits) != 0 {
			if oba.Resources.Limits.Cpu().String() != "0" {
				cpuLimits = oba.Resources.Limits.Cpu().String()
			}
			if oba.Resources.Limits.Memory().String() != "0" {
				memoryLimits = oba.Resources.Limits.Memory().String()
			}
		}
	}

	resourceReq := &corev1.ResourceRequirements{}
	requests := corev1.ResourceList{}
	limits := corev1.ResourceList{}
	if cpuRequests != "" {
		requests[corev1.ResourceName(corev1.ResourceCPU)] = resource.MustParse(cpuRequests)
	}
	if memoryRequests != "" {
		requests[corev1.ResourceName(corev1.ResourceMemory)] = resource.MustParse(memoryRequests)
	}
	if cpuLimits != "" {
		limits[corev1.ResourceName(corev1.ResourceCPU)] = resource.MustParse(cpuLimits)
	}
	if memoryLimits != "" {
		limits[corev1.ResourceName(corev1.ResourceMemory)] = resource.MustParse(memoryLimits)
	}
	resourceReq.Limits = limits
	resourceReq.Requests = requests

	return resourceReq
}
