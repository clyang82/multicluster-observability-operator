// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package config

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	mcoshared "github.com/open-cluster-management/multicluster-observability-operator/operators/multiclusterobservability/api/shared"
	operatorsconfig "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/config"
)

func TestGetOBAResources(t *testing.T) {
	caseList := []struct {
		name          string
		componentName string
		raw           *mcoshared.ObservabilityAddonSpec
		result        func(resources corev1.ResourceRequirements) bool
	}{
		{
			name:          "Have requests defined",
			componentName: operatorsconfig.ObservatoriumAPI,
			raw: &mcoshared.ObservabilityAddonSpec{
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
			},
			result: func(resources corev1.ResourceRequirements) bool {
				return resources.Requests.Cpu().String() == "1" &&
					resources.Requests.Memory().String() == "1Gi" &&
					resources.Limits.Cpu().String() == "0" &&
					resources.Limits.Memory().String() == "0"
			},
		},
		{
			name:          "Have limits defined",
			componentName: operatorsconfig.ObservatoriumAPI,
			raw: &mcoshared.ObservabilityAddonSpec{
				Resources: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("1"),
					},
				},
			},
			result: func(resources corev1.ResourceRequirements) bool {
				return resources.Requests.Cpu().String() == MetricsCollectorCPURequets &&
					resources.Requests.Memory().String() == MetricsCollectorMemoryRequets &&
					resources.Limits.Cpu().String() == "1" &&
					resources.Limits.Memory().String() == "0"
			},
		},
		{
			name:          "no resources defined",
			componentName: operatorsconfig.ObservatoriumAPI,
			raw: &mcoshared.ObservabilityAddonSpec{
				Resources: &corev1.ResourceRequirements{},
			},
			result: func(resources corev1.ResourceRequirements) bool {
				return resources.Requests.Cpu().String() == MetricsCollectorCPURequets &&
					resources.Requests.Memory().String() == MetricsCollectorMemoryRequets &&
					resources.Limits.Cpu().String() == "0" &&
					resources.Limits.Memory().String() == "0"
			},
		},
	}
	for _, c := range caseList {
		t.Run(c.componentName+":"+c.name, func(t *testing.T) {
			resources := GetOBAResources(c.raw)
			if !c.result(*resources) {
				t.Errorf("case (%v) output (%v) is not the expected", c.componentName+":"+c.name, resources)
			}
		})
	}
}
