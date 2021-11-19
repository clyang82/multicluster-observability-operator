// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package config

import (
	"context"
	"fmt"
	"os"
	"strings"

	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	observabilityv1beta2 "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/apis/multiclusterobservability/v1beta2"
)

const (
	ClusterNameKey                  = "cluster-name"
	HubInfoSecretName               = "hub-info-secret"
	ObsAPIGateway                   = "observatorium-api"
	HubInfoSecretKey                = "hub-info.yaml" // #nosec
	ObservatoriumAPIRemoteWritePath = "/api/metrics/v1/default/api/v1/receive"
	AnnotationSkipCreation          = "skip-creation-if-exist"
	defaultMCONamespace             = "open-cluster-management"
	defaultNamespace                = "open-cluster-management-observability"

	DefaultImgRepository = "quay.io/open-cluster-management"
	DefaultImgTagSuffix  = "2.4.0-SNAPSHOT-2021-09-23-07-02-14"

	MCORsName = "multiclusterobservabilities"

	CollectorImage         = "COLLECTOR_IMAGE"
	InstallPrometheus      = "INSTALL_PROM"
	PullSecret             = "PULL_SECRET"
	ImageConfigMap         = "images-list"
	AllowlistConfigMapName = "observability-metrics-allowlist"

	IngressControllerCRD           = "ingresscontrollers.operator.openshift.io"
	MCHCrdName                     = "multiclusterhubs.operator.open-cluster-management.io"
	StorageVersionMigrationCrdName = "storageversionmigrations.migration.k8s.io"
	MCOCrdName                     = "multiclusterobservabilities.observability.open-cluster-management.io"

	DefaultImagePullSecret = "multiclusterhub-operator-pull-secret"
	DefaultImagePullPolicy = "Always"

	AnnotationKeyImageRepository          = "mco-imageRepository"
	AnnotationKeyImageTagSuffix           = "mco-imageTagSuffix"
	AnnotationMCOPause                    = "mco-pause"
	AnnotationMCOWithoutResourcesRequests = "mco-thanos-without-resources-requests"
	AnnotationCertDuration                = "mco-cert-duration"

	ServerCACerts = "observability-server-ca-certs"
	ClientCACerts = "observability-client-ca-certs"

	ImageManifestConfigMapNamePrefix    = "mch-image-manifest-"
	OCMManifestConfigMapTypeLabelKey    = "ocm-configmap-type"
	OCMManifestConfigMapTypeLabelValue  = "image-manifest"
	OCMManifestConfigMapVersionLabelKey = "ocm-release-version"

	OpenshiftIngressOperatorCRName    = "default"
	OpenshiftIngressOperatorNamespace = "openshift-ingress-operator"
	OpenshiftIngressNamespace         = "openshift-ingress"
	OpenshiftIngressDefaultCertName   = "router-certs-default"
	OpenshiftIngressRouteCAName       = "router-ca"

	AlertmanagerRouteBYOCAName       = "alertmanager-byo-ca"
	AlertmanagerRouteBYOCERTName     = "alertmanager-byo-cert"
	AlertmanagerRouteName            = "alertmanager"
	AlertmanagerServiceName          = "alertmanager"
	AlertmanagersDefaultCaBundleName = "alertmanager-ca-bundle"

	MCHUpdatedRequestName = "mch-updated-request"
	MCOUpdatedRequestName = "mco-updated-request"
)

const (
	MetricsCollectorImgName = "metrics-collector"
	MetricsCollectorKey     = "metrics_collector"

	PrometheusImgName = "prometheus"
	PrometheusKey     = "prometheus"

	KubeStateMetricsImgName = "kube-state-metrics"
	KubeStateMetricsKey     = "kube_state_metrics"

	NodeExporterImgName = "node-exporter"
	NodeExporterKey     = "node_exporter"

	KubeRbacProxyImgName = "kube-rbac-proxy"
	KubeRbacProxyKey     = "kube_rbac_proxy"

	ConfigmapReloaderImgRepo      = "quay.io/openshift"
	ConfigmapReloaderImgName      = "origin-configmap-reloader"
	ConfigmapReloaderImgTagSuffix = "4.8.0"
	ConfigmapReloaderKey          = "prometheus-config-reloader"

	ObservatoriumAPI = "observatorium-api"

	operandNamePrefix = "observability-"
)

var (
	log                        = logf.Log.WithName("config")
	monitoringCRName           = ""
	defaultCRName              = "observability"
	imageManifests             = map[string]string{}
	imageManifestConfigMapName = ""
	ImageKeyNameMap            = map[string]string{
		PrometheusKey:        PrometheusKey,
		KubeStateMetricsKey:  KubeStateMetricsImgName,
		NodeExporterKey:      NodeExporterImgName,
		KubeRbacProxyKey:     KubeRbacProxyImgName,
		MetricsCollectorKey:  MetricsCollectorImgName,
		ConfigmapReloaderKey: ConfigmapReloaderImgName,
	}
)

// HubInfo is the struct that contains the common information about the hub
// cluster, for example the name of managed cluster on the hub, the URL of
// observatorium api gateway, the URL of hub alertmanager and the CA for the
// hub router
type HubInfo struct {
	ClusterName              string `yaml:"cluster-name"`
	ObservatoriumAPIEndpoint string `yaml:"observatorium-api-endpoint"`
	AlertmanagerEndpoint     string `yaml:"alertmanager-endpoint"`
	AlertmanagerRouterCA     string `yaml:"alertmanager-router-ca"`
}

// GetDefaultNamespace returns the default obs installed namespace
func GetDefaultNamespace() string {
	return defaultNamespace
}

// GetDefaultMCONamespace returns the default mco operator namespace
func GetDefaultMCONamespace() string {
	return defaultMCONamespace
}

func GetImagePullPolicy(mco observabilityv1beta2.MultiClusterObservabilitySpec) corev1.PullPolicy {
	if mco.ImagePullPolicy != "" {
		return mco.ImagePullPolicy
	} else {
		return DefaultImagePullPolicy
	}
}

func GetImagePullSecret(mco observabilityv1beta2.MultiClusterObservabilitySpec) string {
	if mco.ImagePullSecret != "" {
		return mco.ImagePullSecret
	} else {
		return DefaultImagePullSecret
	}
}

func GetImageManifestConfigMapName() string {
	return imageManifestConfigMapName
}

func GetMCONamespace() string {
	podNamespace, found := os.LookupEnv("MCO_POD_NAMESPACE")
	if !found {
		podNamespace = GetDefaultMCONamespace()
	}
	return podNamespace
}

// ReadImageManifestConfigMap reads configmap with the label ocm-configmap-type=image-manifest
func ReadImageManifestConfigMap(c client.Client, version string) (bool, error) {
	mcoNamespace := GetMCONamespace()
	// List image manifest configmap with label ocm-configmap-type=image-manifest and ocm-release-version
	matchLabels := map[string]string{
		OCMManifestConfigMapTypeLabelKey:    OCMManifestConfigMapTypeLabelValue,
		OCMManifestConfigMapVersionLabelKey: version,
	}
	listOpts := []client.ListOption{
		client.InNamespace(mcoNamespace),
		client.MatchingLabels(matchLabels),
	}

	imageCMList := &corev1.ConfigMapList{}
	err := c.List(context.TODO(), imageCMList, listOpts...)
	if err != nil {
		return false, fmt.Errorf("Failed to list mch-image-manifest configmaps: %v", err)
	}

	if len(imageCMList.Items) != 1 {
		// there should be only one matched image manifest configmap found
		return false, nil
	}

	imageManifests = imageCMList.Items[0].Data
	log.V(1).Info("the length of mch-image-manifest configmap", "imageManifests", len(imageManifests))
	return true, nil
}

// GetImageManifests...
func GetImageManifests() map[string]string {
	return imageManifests
}

// SetImageManifests sets imageManifests
func SetImageManifests(images map[string]string) {
	imageManifests = images
}

// ReplaceImage is used to replace the image with specified annotation or imagemanifest configmap
func ReplaceImage(annotations map[string]string, imageRepo, componentName string) (bool, string) {
	if annotations != nil {
		annotationImageRepo, _ := annotations[AnnotationKeyImageRepository]
		if annotationImageRepo == "" {
			annotationImageRepo = DefaultImgRepository
		}
		// This is for test only. e.g.:
		// if there is "mco-metrics_collector-image" defined in annotation, use it for testing
		componentImage, hasComponentImage := annotations["mco-"+componentName+"-image"]
		tagSuffix, hasTagSuffix := annotations[AnnotationKeyImageTagSuffix]
		sameOrg := strings.Contains(imageRepo, DefaultImgRepository)

		if hasComponentImage {
			return true, componentImage
		} else if hasTagSuffix && sameOrg {
			repoSlice := strings.Split(imageRepo, "/")
			imageName := strings.Split(repoSlice[len(repoSlice)-1], ":")[0]
			image := annotationImageRepo + "/" + imageName + ":" + tagSuffix
			log.V(1).Info("image replacement", "componentName", image)
			return true, image
		} else if !hasTagSuffix {
			image, found := imageManifests[componentName]
			log.V(1).Info("image replacement", "componentName", image)
			if found {
				return true, image
			}
			return false, ""
		}
		return false, ""
	} else {
		image, found := imageManifests[componentName]
		log.V(1).Info("image replacement", "componentName", image)
		if found {
			return true, image
		}
		return false, ""
	}
}

// SetMonitoringCRName sets the cr name
func SetMonitoringCRName(crName string) {
	monitoringCRName = crName
}

// GetDefaultCRName is used to get default CR name.
func GetDefaultCRName() string {
	return defaultCRName
}

// GetMonitoringCRName returns monitoring cr name
func GetMonitoringCRName() string {
	return monitoringCRName
}

// IsPaused returns true if the multiclusterobservability instance is labeled as paused, and false otherwise
func IsPaused(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}

	if annotations[AnnotationMCOPause] != "" &&
		strings.EqualFold(annotations[AnnotationMCOPause], "true") {
		return true
	}

	return false
}

// GetDomainForIngressController get the domain for the given ingresscontroller instance
func GetDomainForIngressController(client client.Client, name, namespace string) (string, error) {
	ingressOperatorInstance := &operatorv1.IngressController{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, ingressOperatorInstance)
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
func GetObsAPIHost(client client.Client, namespace string) (string, error) {
	found := &routev1.Route{}

	err := client.Get(context.TODO(), types.NamespacedName{Name: ObsAPIGateway, Namespace: namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// if the observatorium-api router is not created yet, fallback to get host from the domain of ingresscontroller
		domain, err := GetDomainForIngressController(client, OpenshiftIngressOperatorCRName, OpenshiftIngressOperatorNamespace)
		if err != nil {
			return "", nil
		}
		return ObsAPIGateway + "-" + namespace + "." + domain, nil
	} else if err != nil {
		return "", err
	}
	return found.Spec.Host, nil
}

func GetOperandNamePrefix() string {
	return operandNamePrefix
}
