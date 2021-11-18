// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package config

import (
	"context"

	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mcoshared "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/apis/multiclusterobservability/shared"
	"github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/config"
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

// GetAlertmanagerEndpoint is used to get the URL for alertmanager
func GetAlertmanagerEndpoint(client client.Client, namespace string) (string, error) {
	found := &routev1.Route{}

	err := client.Get(context.TODO(), types.NamespacedName{Name: config.AlertmanagerRouteName, Namespace: namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// if the alertmanager router is not created yet, fallback to get host from the domain of ingresscontroller
		domain, err := config.GetDomainForIngressController(client, config.OpenshiftIngressOperatorCRName, config.OpenshiftIngressOperatorNamespace)
		if err != nil {
			return "", nil
		}
		return config.AlertmanagerRouteName + "-" + namespace + "." + domain, nil
	} else if err != nil {
		return "", err
	}
	return found.Spec.Host, nil
}

// GetAlertmanagerRouterCA is used to get the CA of openshift Route
func GetAlertmanagerRouterCA(client client.Client) (string, error) {
	amRouteBYOCaSrt := &corev1.Secret{}
	amRouteBYOCertSrt := &corev1.Secret{}
	err1 := client.Get(context.TODO(), types.NamespacedName{Name: config.AlertmanagerRouteBYOCAName, Namespace: config.GetDefaultNamespace()}, amRouteBYOCaSrt)
	err2 := client.Get(context.TODO(), types.NamespacedName{Name: config.AlertmanagerRouteBYOCERTName, Namespace: config.GetDefaultNamespace()}, amRouteBYOCertSrt)
	if err1 == nil && err2 == nil {
		return string(amRouteBYOCaSrt.Data["tls.crt"]), nil
	}

	ingressOperator := &operatorv1.IngressController{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: config.OpenshiftIngressOperatorCRName,
		Namespace: config.OpenshiftIngressOperatorNamespace}, ingressOperator)
	if err != nil {
		return "", err
	}

	routerCASrtName := config.OpenshiftIngressDefaultCertName
	// check if custom default certificate is provided or not
	if ingressOperator.Spec.DefaultCertificate != nil {
		routerCASrtName = ingressOperator.Spec.DefaultCertificate.Name
	}

	routerCASecret := &corev1.Secret{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: routerCASrtName, Namespace: config.OpenshiftIngressNamespace}, routerCASecret)
	if err != nil {
		return "", err
	}
	return string(routerCASecret.Data["tls.crt"]), nil
}

// GetAlertmanagerCA is used to get the CA of Alertmanager
func GetAlertmanagerCA(client client.Client) (string, error) {
	amCAConfigmap := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: config.AlertmanagersDefaultCaBundleName, Namespace: config.GetDefaultNamespace()}, amCAConfigmap)
	if err != nil {
		return "", err
	}
	return string(amCAConfigmap.Data["service-ca.crt"]), nil
}
