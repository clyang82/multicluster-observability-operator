// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package config

import (
	"context"
	"os"
	"strings"
	"time"

	obsv1alpha1 "github.com/open-cluster-management/observatorium-operator/api/v1alpha1"
	ocinfrav1 "github.com/openshift/api/config/v1"
	ocpClientSet "github.com/openshift/client-go/config/clientset/versioned"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	observabilityv1beta2 "github.com/open-cluster-management/multicluster-observability-operator/operators/multiclusterobservability/api/v1beta2"
	"github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/config"
)

const (
	crLabelKey               = "observability.open-cluster-management.io/name"
	clusterNameLabelKey      = "cluster"
	infrastructureConfigName = "cluster"
	defaultTenantName        = "default"

	ComponentVersion = "COMPONENT_VERSION"

	ServerCerts      = "observability-server-certs"
	ServerCertCN     = "observability-server-certificate"
	GrafanaCerts     = "observability-grafana-certs"
	GrafanaCN        = "grafana"
	ManagedClusterOU = "acm"

	AlertRuleDefaultConfigMapName = "thanos-ruler-default-rules"
	AlertRuleDefaultFileKey       = "default_rules.yaml"
	AlertRuleCustomConfigMapName  = "thanos-ruler-custom-rules"
	AlertRuleCustomFileKey        = "custom_rules.yaml"
	AlertmanagerURL               = "http://alertmanager:9093"
	AlertmanagerConfigName        = "alertmanager-config"

	AlertmanagersDefaultConfigMapName     = "thanos-ruler-config"
	AlertmanagersDefaultConfigFileKey     = "config.yaml"
	AlertmanagersDefaultCaBundleMountPath = "/etc/thanos/configmaps/alertmanager-ca-bundle"
	AlertmanagersDefaultCaBundleKey       = "service-ca.crt"

	ProxyServiceName      = "rbac-query-proxy"
	ProxyRouteName        = "rbac-query-proxy"
	ProxyRouteBYOCAName   = "proxy-byo-ca"
	ProxyRouteBYOCERTName = "proxy-byo-cert"

	ValidatingWebhookConfigurationName = "multicluster-observability-operator"
	WebhookServiceName                 = "multicluster-observability-webhook-service"
)

const (
	ObservatoriumImgRepo           = "quay.io/observatorium"
	ObservatoriumAPIImgName        = "observatorium"
	ObservatoriumOperatorImgName   = "observatorium-operator"
	ObservatoriumOperatorImgKey    = "observatorium_operator"
	ThanosReceiveControllerImgName = "thanos-receive-controller"
	//ThanosReceiveControllerKey is used to get from mch-image-manifest.xxx configmap
	ThanosReceiveControllerKey    = "thanos_receive_controller"
	ThanosReceiveControllerImgTag = "master-2021-04-28-ee165b6"
	ThanosImgName                 = "thanos"

	MemcachedImgRepo = "quay.io/ocm-observability"
	MemcachedImgName = "memcached"
	MemcachedImgTag  = "1.6.3-alpine"

	MemcachedExporterImgRepo = "quay.io/prometheus"
	MemcachedExporterImgName = "memcached-exporter"
	MemcachedExporterKey     = "memcached_exporter"
	MemcachedExporterImgTag  = "v0.9.0"

	GrafanaImgKey              = "grafana"
	GrafanaDashboardLoaderName = "grafana-dashboard-loader"
	GrafanaDashboardLoaderKey  = "grafana_dashboard_loader"

	AgentImgKey  = "agent_operator"
	AgentImgName = "agent-operator"

	AlertManagerImgName = "prometheus-alertmanager"
	AlertManagerImgKey  = "prometheus_alertmanager"

	OauthProxyImgRepo      = "quay.io/open-cluster-management"
	OauthProxyImgName      = "origin-oauth-proxy"
	OauthProxyImgTagSuffix = "2.0.12-SNAPSHOT-2021-06-11-19-40-10"
	OauthProxyKey          = "oauth_proxy"

	RBACQueryProxyImgName = "rbac-query-proxy"
	RBACQueryProxyKey     = "rbac_query_proxy"

	RBACQueryProxyCPURequets    = "20m"
	RBACQueryProxyMemoryRequets = "100Mi"

	AgentCPURequets    = "10m"
	AgentMemoryRequets = "100Mi"

	GrafanaCPURequets    = "4m"
	GrafanaMemoryRequets = "100Mi"
	GrafanaCPULimits     = "500m"
	GrafanaMemoryLimits  = "1Gi"

	AlertmanagerCPURequets    = "4m"
	AlertmanagerMemoryRequets = "200Mi"

	ObservatoriumAPICPURequets    = "20m"
	ObservatoriumAPIMemoryRequets = "128Mi"

	ThanosQueryFrontendCPURequets    = "100m"
	ThanosQueryFrontendMemoryRequets = "256Mi"

	MemcachedExporterCPURequets    = "5m"
	MemcachedExporterMemoryRequets = "50Mi"

	ThanosQueryCPURequets    = "300m"
	ThanosQueryMemoryRequets = "1Gi"

	ThanosCompactCPURequets    = "100m"
	ThanosCompactMemoryRequets = "512Mi"

	ObservatoriumReceiveControllerCPURequets    = "4m"
	ObservatoriumReceiveControllerMemoryRequets = "32Mi"

	ThanosReceiveCPURequets    = "300m"
	ThanosReceiveMemoryRequets = "512Mi"

	ThanosRuleCPURequets            = "50m"
	ThanosRuleMemoryRequets         = "512Mi"
	ThanosRuleReloaderCPURequets    = "4m"
	ThanosRuleReloaderMemoryRequets = "25Mi"

	ThanosCachedCPURequets            = "45m"
	ThanosCachedMemoryRequets         = "128Mi"
	ThanosCachedExporterCPURequets    = "5m"
	ThanosCachedExporterMemoryRequets = "50Mi"

	ThanosStoreCPURequets    = "100m"
	ThanosStoreMemoryRequets = "1Gi"

	ThanosCompact                = "thanos-compact"
	ThanosQuery                  = "thanos-query"
	ThanosQueryFrontend          = "thanos-query-frontend"
	ThanosQueryFrontendMemcached = "thanos-query-frontend-memcached"
	ThanosRule                   = "thanos-rule"
	ThanosReceive                = "thanos-receive-default"
	ThanosStoreMemcached         = "thanos-store-memcached"
	ThanosStoreShard             = "thanos-store-shard"
	MemcachedExporter            = "memcached-exporter"
	Grafana                      = "grafana"
	AgentOperator                = "agent-operator"
	RBACQueryProxy               = "rbac-query-proxy"
	Alertmanager                 = "alertmanager"
	ThanosReceiveController      = "thanos-receive-controller"
	ObservatoriumOperator        = "observatorium-operator"
	MetricsCollector             = "metrics-collector"
	Observatorium                = "observatorium"

	RetentionResolutionRaw = "30d"
	RetentionResolution5m  = "180d"
	RetentionResolution1h  = "0d"
	RetentionInLocal       = "24h"
	DeleteDelay            = "48h"
	BlockDuration          = "2h"

	ResourceLimits   = "limits"
	ResourceRequests = "requests"
)

// ObjectStorgeConf is used to Unmarshal from bytes to do validation
type ObjectStorgeConf struct {
	Type   string `yaml:"type"`
	Config Config `yaml:"config"`
}

var (
	log                         = logf.Log.WithName("multiclusterobservability-config")
	tenantUID                   = ""
	hasCustomRuleConfigMap      = false
	hasCustomAlertmanagerConfig = false
	certDuration                = time.Hour * 24 * 365

	Replicas1 int32 = 1
	Replicas2 int32 = 2
	Replicas3 int32 = 3
	Replicas        = map[string]*int32{
		config.ObservatoriumAPI: &Replicas2,
		ThanosQuery:             &Replicas2,
		ThanosQueryFrontend:     &Replicas2,
		Grafana:                 &Replicas2,
		RBACQueryProxy:          &Replicas2,

		ThanosRule:                   &Replicas3,
		ThanosReceive:                &Replicas3,
		ThanosStoreShard:             &Replicas3,
		ThanosStoreMemcached:         &Replicas3,
		ThanosQueryFrontendMemcached: &Replicas3,
		Alertmanager:                 &Replicas3,
	}
	// use this map to store the operand name
	operandNames = map[string]string{}

	MemoryLimitMB   = int32(1024)
	ConnectionLimit = int32(1024)
	MaxItemSize     = "1m"
)

func GetReplicas(component string, advanced *observabilityv1beta2.AdvancedConfig) *int32 {
	if advanced == nil {
		return Replicas[component]
	}
	var replicas *int32
	switch component {
	case config.ObservatoriumAPI:
		if advanced.ObservatoriumAPI != nil {
			replicas = advanced.ObservatoriumAPI.Replicas
		}
	case ThanosQuery:
		if advanced.Query != nil {
			replicas = advanced.Query.Replicas
		}
	case ThanosQueryFrontend:
		if advanced.QueryFrontend != nil {
			replicas = advanced.QueryFrontend.Replicas
		}
	case ThanosQueryFrontendMemcached:
		if advanced.QueryFrontendMemcached != nil {
			replicas = advanced.QueryFrontendMemcached.CommonSpec.Replicas
		}
	case ThanosRule:
		if advanced.Rule != nil {
			replicas = advanced.Rule.Replicas
		}
	case ThanosReceive:
		if advanced.Receive != nil {
			replicas = advanced.Receive.Replicas
		}
	case ThanosStoreMemcached:
		if advanced.StoreMemcached != nil {
			replicas = advanced.StoreMemcached.CommonSpec.Replicas
		}
	case ThanosStoreShard:
		if advanced.Store != nil {
			replicas = advanced.Store.Replicas
		}
	case RBACQueryProxy:
		if advanced.RBACQueryProxy != nil {
			replicas = advanced.RBACQueryProxy.Replicas
		}
	case Grafana:
		if advanced.Grafana != nil {
			replicas = advanced.Grafana.Replicas
		}
	case Alertmanager:
		if advanced.Alertmanager != nil {
			replicas = advanced.Alertmanager.Replicas
		}
	}
	if replicas == nil || *replicas == 0 {
		replicas = Replicas[component]
	}
	return replicas
}

// GetCrLabelKey returns the key for the CR label injected into the resources created by the operator
func GetCrLabelKey() string {
	return crLabelKey
}

// GetClusterNameLabelKey returns the key for the injected label
func GetClusterNameLabelKey() string {
	return clusterNameLabelKey
}

// GetDefaultTenantName returns the default tenant name
func GetDefaultTenantName() string {
	return defaultTenantName
}

func GetMCONamespace() string {
	podNamespace, found := os.LookupEnv("POD_NAMESPACE")
	if !found {
		podNamespace = config.GetDefaultMCONamespace()
	}
	return podNamespace
}

func infrastructureConfigNameNsN() types.NamespacedName {
	return types.NamespacedName{
		Name: infrastructureConfigName,
	}
}

// GetKubeAPIServerAddress is used to get the api server url
func GetKubeAPIServerAddress(client client.Client) (string, error) {
	infraConfig := &ocinfrav1.Infrastructure{}
	if err := client.Get(context.TODO(), infrastructureConfigNameNsN(), infraConfig); err != nil {
		return "", err
	}

	return infraConfig.Status.APIServerURL, nil
}

// GetClusterID is used to get the cluster uid
func GetClusterID(ocpClient ocpClientSet.Interface) (string, error) {
	clusterVersion, err := ocpClient.ConfigV1().ClusterVersions().Get(context.TODO(), "version", v1.GetOptions{})
	if err != nil {
		log.Error(err, "Failed to get clusterVersion")
		return "", err
	}

	return string(clusterVersion.Spec.ClusterID), nil
}

// checkIsIBMCloud detects if the current cloud vendor is ibm or not
// we know we are on OCP already, so if it's also ibm cloud, it's roks
func CheckIsIBMCloud(c client.Client) (bool, error) {
	nodes := &corev1.NodeList{}
	err := c.List(context.TODO(), nodes)
	if err != nil {
		log.Error(err, "Failed to get nodes list")
		return false, err
	}
	if len(nodes.Items) == 0 {
		log.Error(err, "Failed to list any nodes")
		return false, nil
	}

	providerID := nodes.Items[0].Spec.ProviderID
	if strings.Contains(providerID, "ibm") {
		return true, nil
	}

	return false, nil
}

// WithoutResourcesRequests returns true if the multiclusterobservability instance has annotation:
// mco-thanos-without-resources-requests: "true"
// This is just for test purpose: the KinD cluster does not have enough resources for the requests.
// We won't expose this annotation to the customer.
func WithoutResourcesRequests(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}

	if annotations[config.AnnotationMCOWithoutResourcesRequests] != "" &&
		strings.EqualFold(annotations[config.AnnotationMCOWithoutResourcesRequests], "true") {
		return true
	}

	return false
}

// GetTenantUID returns tenant uid
func GetTenantUID() string {
	if tenantUID == "" {
		tenantUID = string(uuid.NewUUID())
	}
	return tenantUID
}

// GetObsAPISvc returns observatorium api service
func GetObsAPISvc(instanceName string) string {
	return instanceName + "-observatorium-api." + config.GetDefaultNamespace() + ".svc.cluster.local"
}

// SetCustomRuleConfigMap set true if there is custom rule configmap
func SetCustomRuleConfigMap(hasConfigMap bool) {
	hasCustomRuleConfigMap = hasConfigMap
}

// HasCustomRuleConfigMap returns true if there is custom rule configmap
func HasCustomRuleConfigMap() bool {
	return hasCustomRuleConfigMap
}

func GetCertDuration() time.Duration {
	return certDuration
}

func SetCertDuration(annotations map[string]string) {
	if annotations != nil && annotations[config.AnnotationCertDuration] != "" {
		d, err := time.ParseDuration(annotations[config.AnnotationCertDuration])
		if err != nil {
			log.Error(err, "Failed to parse cert duration, use default one", "annotation", annotations[config.AnnotationCertDuration])
		} else {
			certDuration = d
			return
		}
	}
	certDuration = time.Hour * 24 * 365
}

func getDefaultResource(resourceType string, resource corev1.ResourceName,
	component string) string {
	//No provide the default limits
	if resourceType == ResourceLimits && component != Grafana {
		return ""
	}
	switch component {
	case config.ObservatoriumAPI:
		if resource == corev1.ResourceCPU {
			return ObservatoriumAPICPURequets
		}
		if resource == corev1.ResourceMemory {
			return ObservatoriumAPIMemoryRequets
		}
	case ThanosCompact:
		if resource == corev1.ResourceCPU {
			return ThanosCompactCPURequets
		}
		if resource == corev1.ResourceMemory {
			return ThanosCompactMemoryRequets
		}
	case ThanosQuery:
		if resource == corev1.ResourceCPU {
			return ThanosQueryCPURequets
		}
		if resource == corev1.ResourceMemory {
			return ThanosQueryMemoryRequets
		}
	case ThanosQueryFrontend:
		if resource == corev1.ResourceCPU {
			return ThanosQueryFrontendCPURequets
		}
		if resource == corev1.ResourceMemory {
			return ThanosQueryFrontendMemoryRequets
		}
	case ThanosRule:
		if resource == corev1.ResourceCPU {
			return ThanosRuleCPURequets
		}
		if resource == corev1.ResourceMemory {
			return ThanosRuleMemoryRequets
		}
	case ThanosReceive:
		if resource == corev1.ResourceCPU {
			return ThanosReceiveCPURequets
		}
		if resource == corev1.ResourceMemory {
			return ThanosReceiveMemoryRequets
		}
	case ThanosStoreShard:
		if resource == corev1.ResourceCPU {
			return ThanosStoreCPURequets
		}
		if resource == corev1.ResourceMemory {
			return ThanosStoreMemoryRequets
		}
	case ThanosQueryFrontendMemcached, ThanosStoreMemcached:
		if resource == corev1.ResourceCPU {
			return ThanosCachedCPURequets
		}
		if resource == corev1.ResourceMemory {
			return ThanosCachedMemoryRequets
		}
	case MemcachedExporter:
		if resource == corev1.ResourceCPU {
			return MemcachedExporterCPURequets
		}
		if resource == corev1.ResourceMemory {
			return MemcachedExporterMemoryRequets
		}
	case RBACQueryProxy:
		if resource == corev1.ResourceCPU {
			return RBACQueryProxyCPURequets
		}
		if resource == corev1.ResourceMemory {
			return RBACQueryProxyMemoryRequets
		}
	// case MetricsCollector:
	// 	if resource == corev1.ResourceCPU {
	// 		return MetricsCollectorCPURequets
	// 	}
	// 	if resource == corev1.ResourceMemory {
	// 		return MetricsCollectorMemoryRequets
	// 	}
	case Alertmanager:
		if resource == corev1.ResourceCPU {
			return AlertmanagerCPURequets
		}
		if resource == corev1.ResourceMemory {
			return AlertmanagerMemoryRequets
		}
	case Grafana:
		if resourceType == ResourceRequests {
			if resource == corev1.ResourceCPU {
				return GrafanaCPURequets
			}
			if resource == corev1.ResourceMemory {
				return GrafanaMemoryRequets
			}
		} else if resourceType == ResourceLimits {
			if resource == corev1.ResourceCPU {
				return GrafanaCPULimits
			}
			if resource == corev1.ResourceMemory {
				return GrafanaMemoryLimits
			}
		}
	}
	return ""
}

func getResource(resourceType string, resource corev1.ResourceName,
	component string, advanced *observabilityv1beta2.AdvancedConfig) string {
	if advanced == nil {
		return getDefaultResource(resourceType, resource, component)
	}
	var resourcesReq *corev1.ResourceRequirements
	switch component {
	case config.ObservatoriumAPI:
		if advanced.ObservatoriumAPI != nil {
			resourcesReq = advanced.ObservatoriumAPI.Resources
		}
	case ThanosCompact:
		if advanced.Compact != nil {
			resourcesReq = advanced.Compact.Resources
		}
	case ThanosQuery:
		if advanced.Query != nil {
			resourcesReq = advanced.Query.Resources
		}
	case ThanosQueryFrontend:
		if advanced.QueryFrontend != nil {
			resourcesReq = advanced.QueryFrontend.Resources
		}
	case ThanosQueryFrontendMemcached:
		if advanced.QueryFrontendMemcached != nil {
			resourcesReq = advanced.QueryFrontendMemcached.CommonSpec.Resources
		}
	case ThanosRule:
		if advanced.Rule != nil {
			resourcesReq = advanced.Rule.Resources
		}
	case ThanosReceive:
		if advanced.Receive != nil {
			resourcesReq = advanced.Receive.Resources
		}
	case ThanosStoreMemcached:
		if advanced.StoreMemcached != nil {
			resourcesReq = advanced.StoreMemcached.CommonSpec.Resources
		}
	case ThanosStoreShard:
		if advanced.Store != nil {
			resourcesReq = advanced.Store.Resources
		}
	case RBACQueryProxy:
		if advanced.RBACQueryProxy != nil {
			resourcesReq = advanced.RBACQueryProxy.Resources
		}
	case Grafana:
		if advanced.Grafana != nil {
			resourcesReq = advanced.Grafana.Resources
		}
	case Alertmanager:
		if advanced.Alertmanager != nil {
			resourcesReq = advanced.Alertmanager.Resources
		}
	}

	if resourcesReq != nil {
		if resourceType == ResourceRequests {
			if len(resourcesReq.Requests) != 0 {
				if resource == corev1.ResourceCPU {
					return resourcesReq.Requests.Cpu().String()
				} else if resource == corev1.ResourceMemory {
					return resourcesReq.Requests.Memory().String()
				} else {
					return getDefaultResource(resourceType, resource, component)
				}
			} else {
				return getDefaultResource(resourceType, resource, component)
			}
		}
		if resourceType == ResourceLimits {
			if len(resourcesReq.Limits) != 0 {
				if resource == corev1.ResourceCPU {
					return resourcesReq.Limits.Cpu().String()
				} else if resource == corev1.ResourceMemory {
					return resourcesReq.Limits.Memory().String()
				} else {
					return getDefaultResource(resourceType, resource, component)
				}
			} else {
				return getDefaultResource(resourceType, resource, component)
			}
		}
	} else {
		return getDefaultResource(resourceType, resource, component)
	}
	return ""
}

func GetResources(component string, advanced *observabilityv1beta2.AdvancedConfig) corev1.ResourceRequirements {

	cpuRequests := getResource(ResourceRequests, corev1.ResourceCPU, component, advanced)
	cpuLimits := getResource(ResourceLimits, corev1.ResourceCPU, component, advanced)
	memoryRequests := getResource(ResourceRequests, corev1.ResourceMemory, component, advanced)
	memoryLimits := getResource(ResourceLimits, corev1.ResourceMemory, component, advanced)

	resourceReq := corev1.ResourceRequirements{}
	requests := corev1.ResourceList{}
	limits := corev1.ResourceList{}
	if cpuRequests == "0" {
		cpuRequests = getDefaultResource(ResourceRequests, corev1.ResourceCPU, component)
	}
	if cpuRequests != "" {
		requests[corev1.ResourceName(corev1.ResourceCPU)] = resource.MustParse(cpuRequests)
	}

	if memoryRequests == "0" {
		memoryRequests = getDefaultResource(ResourceRequests, corev1.ResourceMemory, component)
	}
	if memoryRequests != "" {
		requests[corev1.ResourceName(corev1.ResourceMemory)] = resource.MustParse(memoryRequests)
	}

	if cpuLimits == "0" {
		cpuLimits = getDefaultResource(ResourceLimits, corev1.ResourceCPU, component)
	}
	if cpuLimits != "" {
		limits[corev1.ResourceName(corev1.ResourceCPU)] = resource.MustParse(cpuLimits)
	}

	if memoryLimits == "0" {
		memoryLimits = getDefaultResource(ResourceLimits, corev1.ResourceMemory, component)
	}
	if memoryLimits != "" {
		limits[corev1.ResourceName(corev1.ResourceMemory)] = resource.MustParse(memoryLimits)
	}
	resourceReq.Limits = limits
	resourceReq.Requests = requests

	return resourceReq
}

func GetOperandName(name string) string {
	log.V(1).Info("operand is", "key", name, "name", operandNames[name])
	return operandNames[name]
}

func SetOperandNames(c client.Client) error {
	if len(operandNames) != 0 {
		return nil
	}
	//set the default values.
	operandNames[Grafana] = config.GetOperandNamePrefix() + Grafana
	operandNames[RBACQueryProxy] = config.GetOperandNamePrefix() + RBACQueryProxy
	operandNames[Alertmanager] = config.GetOperandNamePrefix() + Alertmanager
	operandNames[ObservatoriumOperator] = config.GetOperandNamePrefix() + ObservatoriumOperator
	operandNames[AgentOperator] = config.GetOperandNamePrefix() + AgentOperator
	operandNames[Observatorium] = config.GetDefaultCRName()
	operandNames[config.ObservatoriumAPI] = config.GetOperandNamePrefix() + config.ObservatoriumAPI

	// Check if the Observatorium CR already exists
	opts := &client.ListOptions{
		Namespace: config.GetDefaultNamespace(),
	}

	observatoriumList := &obsv1alpha1.ObservatoriumList{}
	err := c.List(context.TODO(), observatoriumList, opts)
	if err != nil {
		return err
	}
	if len(observatoriumList.Items) != 0 {
		for _, observatorium := range observatoriumList.Items {
			for _, ownerRef := range observatorium.OwnerReferences {
				if ownerRef.Kind == "MultiClusterObservability" && ownerRef.Name == config.GetMonitoringCRName() {
					if observatorium.Name != config.GetDefaultCRName() {
						// this is for upgrade case.
						operandNames[Grafana] = Grafana
						operandNames[RBACQueryProxy] = RBACQueryProxy
						operandNames[Alertmanager] = Alertmanager
						operandNames[ObservatoriumOperator] = ObservatoriumOperator
						operandNames[Observatorium] = observatorium.Name
						operandNames[config.ObservatoriumAPI] = observatorium.Name + "-" + config.ObservatoriumAPI
					}
					break
				}
			}
		}
	}

	return nil
}

// CleanUpOperandNames delete all the operand name items
// Should be called when the MCO CR is deleted
func CleanUpOperandNames() {
	for k := range operandNames {
		delete(operandNames, k)
	}
}

// GetValidatingWebhookConfigurationForMCO return the ValidatingWebhookConfiguration for the MCO validaing webhook
func GetValidatingWebhookConfigurationForMCO() *admissionregistrationv1.ValidatingWebhookConfiguration {
	validatingWebhookPath := "/validate-observability-open-cluster-management-io-v1beta2-multiclusterobservability"
	noSideEffects := admissionregistrationv1.SideEffectClassNone
	allScopeType := admissionregistrationv1.AllScopes
	webhookServiceNamespace := GetMCONamespace()
	webhookServicePort := int32(443)
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: ValidatingWebhookConfigurationName,
			Labels: map[string]string{
				"name": ValidatingWebhookConfigurationName,
			},
			Annotations: map[string]string{
				"service.beta.openshift.io/inject-cabundle": "true",
			},
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				Name:                    "vmulticlusterobservability.observability.open-cluster-management.io",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Name:      WebhookServiceName,
						Namespace: webhookServiceNamespace,
						Path:      &validatingWebhookPath,
						Port:      &webhookServicePort,
					},
					CABundle: []byte(""),
				},
				SideEffects: &noSideEffects,
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{
							admissionregistrationv1.Create,
							admissionregistrationv1.Update,
						},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"observability.open-cluster-management.io"},
							APIVersions: []string{"v1beta2"},
							Resources:   []string{"multiclusterobservabilities"},
							Scope:       &allScopeType,
						},
					},
				},
			},
		},
	}
}
