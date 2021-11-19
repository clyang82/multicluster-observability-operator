// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package config

import (
	"context"
	"fmt"

	operatorv1 "github.com/openshift/api/operator/v1"
	operatorclientset "github.com/openshift/client-go/operator/clientset/versioned"
	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	OBSCoreKubeconfigPath = "/observability/core-kubeconfig/kubeconfig"
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
func GetAlertmanagerEndpoint(client routeclientset.Clientset,
	operatorClient operatorclientset.Clientset, namespace string) (string, error) {

	alertRoute, err := client.RouteV1().Routes(namespace).Get(context.TODO(), config.AlertmanagerRouteName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		// if the alertmanager router is not created yet, fallback to get host from the domain of ingresscontroller
		domain, err := getDomainForIngressController(operatorClient, config.OpenshiftIngressOperatorCRName, config.OpenshiftIngressOperatorNamespace)
		if err != nil {
			return "", nil
		}
		return config.AlertmanagerRouteName + "-" + namespace + "." + domain, nil
	} else if err != nil {
		return "", err
	}
	return alertRoute.Spec.Host, nil
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

// getDomainForIngressController get the domain for the given ingresscontroller instance
func getDomainForIngressController(operatorClient operatorclientset.Clientset, name, namespace string) (string, error) {
	ingressOperatorInstance, err := operatorClient.OperatorV1().IngressControllers(namespace).Get(context.TODO(), name, metav1.GetOptions{})

	if err != nil {
		return "", err
	}
	domain := ingressOperatorInstance.Status.Domain
	if domain == "" {
		return "", fmt.Errorf("no domain found in the ingressOperator: %s/%s.", namespace, name)
	}
	return domain, nil
}

// GetObsAPIHost is used to get the URL for observartium api gateway
func GetObsAPIHost(client routeclientset.Clientset,
	operatorClient operatorclientset.Clientset, namespace string) (string, error) {

	obsRoute, err := client.RouteV1().Routes(namespace).Get(context.TODO(), config.ObsAPIGateway, metav1.GetOptions{})

	if err != nil && errors.IsNotFound(err) {
		// if the observatorium-api router is not created yet, fallback to get host from the domain of ingresscontroller
		domain, err := getDomainForIngressController(operatorClient, config.OpenshiftIngressOperatorCRName, config.OpenshiftIngressOperatorNamespace)
		if err != nil {
			return "", nil
		}
		return config.ObsAPIGateway + "-" + namespace + "." + domain, nil
	} else if err != nil {
		return "", err
	}
	return obsRoute.Spec.Host, nil
}
