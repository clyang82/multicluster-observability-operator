// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package config

import (
	"reflect"
	"testing"

	observatoriumv1alpha1 "github.com/open-cluster-management/observatorium-operator/api/v1alpha1"
	configv1 "github.com/openshift/api/config/v1"
	fakeconfigclient "github.com/openshift/client-go/config/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mcoshared "github.com/open-cluster-management/multicluster-observability-operator/operators/multiclusterobservability/api/shared"
	mcov1beta2 "github.com/open-cluster-management/multicluster-observability-operator/operators/multiclusterobservability/api/v1beta2"
	"github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/config"
)

var (
	apiServerURL           = "http://example.com"
	clusterID              = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
	version                = "2.1.1"
	DefaultDSImgRepository = "quay.io:443/acm-d"
)

func TestGetClusterNameLabelKey(t *testing.T) {
	clusterName := GetClusterNameLabelKey()
	if clusterName != clusterNameLabelKey {
		t.Errorf("Cluster Label (%v) is not the expected (%v)", clusterName, clusterNameLabelKey)
	}
}

func TestGetDefaultTenantName(t *testing.T) {
	tenantName := GetDefaultTenantName()
	if tenantName != defaultTenantName {
		t.Errorf("Tenant name (%v) is not the expected (%v)", tenantName, defaultTenantName)
	}
}

func TestGetKubeAPIServerAddress(t *testing.T) {
	inf := &configv1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{Name: infrastructureConfigName},
		Status: configv1.InfrastructureStatus{
			APIServerURL: apiServerURL,
		},
	}
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(configv1.GroupVersion, inf)
	client := fake.NewFakeClientWithScheme(scheme, inf)
	apiURL, _ := GetKubeAPIServerAddress(client)
	if apiURL != apiServerURL {
		t.Errorf("Kubenetes API Server Address (%v) is not the expected (%v)", apiURL, apiServerURL)
	}
}

func TestGetClusterIDSuccess(t *testing.T) {
	version := &configv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "version"},
		Spec: configv1.ClusterVersionSpec{
			ClusterID: configv1.ClusterID(clusterID),
		},
	}
	client := fakeconfigclient.NewSimpleClientset(version)
	tmpClusterID, _ := GetClusterID(client)
	if tmpClusterID != clusterID {
		t.Errorf("OCP ClusterID (%v) is not the expected (%v)", tmpClusterID, clusterID)
	}
}

func TestGetClusterIDFailed(t *testing.T) {
	inf := &configv1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{Name: infrastructureConfigName},
		Status: configv1.InfrastructureStatus{
			APIServerURL: apiServerURL,
		},
	}
	client := fakeconfigclient.NewSimpleClientset(inf)
	_, err := GetClusterID(client)
	if err == nil {
		t.Errorf("Should throw the error since there is no clusterversion defined")
	}
}

func NewFakeClient(mco *mcov1beta2.MultiClusterObservability,
	obs *observatoriumv1alpha1.Observatorium) client.Client {
	s := scheme.Scheme
	s.AddKnownTypes(mcov1beta2.GroupVersion, mco)
	s.AddKnownTypes(observatoriumv1alpha1.GroupVersion, obs)
	objs := []runtime.Object{mco, obs}
	return fake.NewFakeClientWithScheme(s, objs...)
}

func Test_checkIsIBMCloud(t *testing.T) {
	s := scheme.Scheme
	nodeIBM := &corev1.Node{
		Spec: corev1.NodeSpec{
			ProviderID: "ibm",
		},
	}
	nodeOther := &corev1.Node{}

	type args struct {
		client client.Client
		name   string
	}
	caselist := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "is normal ocp",
			args: args{
				client: fake.NewFakeClientWithScheme(s, []runtime.Object{nodeOther}...),
				name:   "test-secret",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "is ibm",
			args: args{
				client: fake.NewFakeClientWithScheme(s, []runtime.Object{nodeIBM}...),
				name:   "test-secret",
			},
			want:    true,
			wantErr: false,
		},
	}
	for _, c := range caselist {
		t.Run(c.name, func(t *testing.T) {
			got, err := CheckIsIBMCloud(c.args.client)
			if (err != nil) != c.wantErr {
				t.Errorf("checkIsIBMCloud() error = %v, wantErr %v", err, c.wantErr)
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("checkIsIBMCloud() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestGetResources(t *testing.T) {
	caseList := []struct {
		name          string
		componentName string
		raw           *mcov1beta2.AdvancedConfig
		result        func(resources corev1.ResourceRequirements) bool
	}{
		{
			name:          "Have requests defined in resources",
			componentName: config.ObservatoriumAPI,
			raw: &mcov1beta2.AdvancedConfig{
				ObservatoriumAPI: &mcov1beta2.CommonSpec{
					Resources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
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
			name:          "Have limits defined in resources",
			componentName: config.ObservatoriumAPI,
			raw: &mcov1beta2.AdvancedConfig{
				ObservatoriumAPI: &mcov1beta2.CommonSpec{
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
			result: func(resources corev1.ResourceRequirements) bool {
				return resources.Requests.Cpu().String() == ObservatoriumAPICPURequets &&
					resources.Requests.Memory().String() == ObservatoriumAPIMemoryRequets &&
					resources.Limits.Cpu().String() == "1" &&
					resources.Limits.Memory().String() == "1Gi"
			},
		},
		{
			name:          "Have limits defined in resources",
			componentName: RBACQueryProxy,
			raw: &mcov1beta2.AdvancedConfig{
				RBACQueryProxy: &mcov1beta2.CommonSpec{
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
			result: func(resources corev1.ResourceRequirements) bool {
				return resources.Requests.Cpu().String() == RBACQueryProxyCPURequets &&
					resources.Requests.Memory().String() == RBACQueryProxyMemoryRequets &&
					resources.Limits.Cpu().String() == "1" &&
					resources.Limits.Memory().String() == "1Gi"
			},
		},
		{
			name:          "Have requests and limits defined in requests",
			componentName: config.ObservatoriumAPI,
			raw: &mcov1beta2.AdvancedConfig{
				ObservatoriumAPI: &mcov1beta2.CommonSpec{
					Resources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
			result: func(resources corev1.ResourceRequirements) bool {
				return resources.Requests.Cpu().String() == "1" &&
					resources.Requests.Memory().String() == "1Gi" &&
					resources.Limits.Cpu().String() == "1" &&
					resources.Limits.Memory().String() == "1Gi"
			},
		},
		{
			name:          "No CPU defined in requests",
			componentName: config.ObservatoriumAPI,
			raw: &mcov1beta2.AdvancedConfig{
				ObservatoriumAPI: &mcov1beta2.CommonSpec{
					Resources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{},
					},
				},
			},
			result: func(resources corev1.ResourceRequirements) bool {
				return resources.Requests.Cpu().String() == ObservatoriumAPICPURequets &&
					resources.Requests.Memory().String() == ObservatoriumAPIMemoryRequets &&
					resources.Limits.Cpu().String() == "0" && resources.Limits.Memory().String() == "0"
			},
		},
		{
			name:          "No requests defined in resources",
			componentName: config.ObservatoriumAPI,
			raw: &mcov1beta2.AdvancedConfig{
				ObservatoriumAPI: &mcov1beta2.CommonSpec{
					Resources: &corev1.ResourceRequirements{},
				},
			},
			result: func(resources corev1.ResourceRequirements) bool {
				return resources.Requests.Cpu().String() == ObservatoriumAPICPURequets &&
					resources.Requests.Memory().String() == ObservatoriumAPIMemoryRequets &&
					resources.Limits.Cpu().String() == "0" && resources.Limits.Memory().String() == "0"
			},
		},
		{
			name:          "No resources defined",
			componentName: config.ObservatoriumAPI,
			raw: &mcov1beta2.AdvancedConfig{
				ObservatoriumAPI: &mcov1beta2.CommonSpec{},
			},
			result: func(resources corev1.ResourceRequirements) bool {
				return resources.Requests.Cpu().String() == ObservatoriumAPICPURequets &&
					resources.Requests.Memory().String() == ObservatoriumAPIMemoryRequets &&
					resources.Limits.Cpu().String() == "0" && resources.Limits.Memory().String() == "0"
			},
		},
		{
			name:          "No advanced defined",
			componentName: config.ObservatoriumAPI,
			raw:           nil,
			result: func(resources corev1.ResourceRequirements) bool {
				return resources.Requests.Cpu().String() == ObservatoriumAPICPURequets &&
					resources.Requests.Memory().String() == ObservatoriumAPIMemoryRequets &&
					resources.Limits.Cpu().String() == "0" && resources.Limits.Memory().String() == "0"
			},
		},
		{
			name:          "No advanced defined",
			componentName: Grafana,
			raw:           nil,
			result: func(resources corev1.ResourceRequirements) bool {
				return resources.Requests.Cpu().String() == GrafanaCPURequets &&
					resources.Requests.Memory().String() == GrafanaMemoryRequets &&
					resources.Limits.Cpu().String() == GrafanaCPULimits &&
					resources.Limits.Memory().String() == GrafanaMemoryLimits
			},
		},
		{
			name:          "Have requests defined",
			componentName: Grafana,
			raw: &mcov1beta2.AdvancedConfig{
				Grafana: &mcov1beta2.CommonSpec{
					Resources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
				},
			},
			result: func(resources corev1.ResourceRequirements) bool {
				return resources.Requests.Cpu().String() == "1" &&
					resources.Requests.Memory().String() == GrafanaMemoryRequets &&
					resources.Limits.Cpu().String() == GrafanaCPULimits &&
					resources.Limits.Memory().String() == GrafanaMemoryLimits
			},
		},
		{
			name:          "Have limits defined",
			componentName: Grafana,
			raw: &mcov1beta2.AdvancedConfig{
				Grafana: &mcov1beta2.CommonSpec{
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
				},
			},
			result: func(resources corev1.ResourceRequirements) bool {
				return resources.Requests.Cpu().String() == GrafanaCPURequets &&
					resources.Requests.Memory().String() == GrafanaMemoryRequets &&
					resources.Limits.Cpu().String() == "1" &&
					resources.Limits.Memory().String() == GrafanaMemoryLimits
			},
		},
		{
			name:          "Have limits defined",
			componentName: Grafana,
			raw: &mcov1beta2.AdvancedConfig{
				Grafana: &mcov1beta2.CommonSpec{
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
			result: func(resources corev1.ResourceRequirements) bool {
				return resources.Requests.Cpu().String() == GrafanaCPURequets &&
					resources.Requests.Memory().String() == GrafanaMemoryRequets &&
					resources.Limits.Cpu().String() == "1" &&
					resources.Limits.Memory().String() == "1Gi"
			},
		},
		{
			name:          "Have limits defined",
			componentName: ThanosQueryFrontendMemcached,
			raw: &mcov1beta2.AdvancedConfig{
				QueryFrontendMemcached: &mcov1beta2.CacheConfig{
					CommonSpec: mcov1beta2.CommonSpec{
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
			result: func(resources corev1.ResourceRequirements) bool {
				return resources.Requests.Cpu().String() == ThanosCachedCPURequets &&
					resources.Requests.Memory().String() == ThanosCachedMemoryRequets &&
					resources.Limits.Cpu().String() == "1" &&
					resources.Limits.Memory().String() == "1Gi"
			},
		},
	}

	for _, c := range caseList {
		t.Run(c.componentName+":"+c.name, func(t *testing.T) {
			resources := GetResources(c.componentName, c.raw)
			if !c.result(resources) {
				t.Errorf("case (%v) output (%v) is not the expected", c.componentName+":"+c.name, resources)
			}
		})
	}
}

func TestGetReplicas(t *testing.T) {
	var replicas0 int32 = 0
	caseList := []struct {
		name          string
		componentName string
		raw           *mcov1beta2.AdvancedConfig
		result        func(replicas *int32) bool
	}{
		{
			name:          "Have replicas defined",
			componentName: config.ObservatoriumAPI,
			raw: &mcov1beta2.AdvancedConfig{
				ObservatoriumAPI: &mcov1beta2.CommonSpec{
					Replicas: &Replicas1,
				},
			},
			result: func(replicas *int32) bool {
				return replicas == &Replicas1
			},
		},
		{
			name:          "Do not allow to set 0",
			componentName: config.ObservatoriumAPI,
			raw: &mcov1beta2.AdvancedConfig{
				ObservatoriumAPI: &mcov1beta2.CommonSpec{
					Replicas: &replicas0,
				},
			},
			result: func(replicas *int32) bool {
				return replicas == &Replicas2
			},
		},
		{
			name:          "No advanced defined",
			componentName: config.ObservatoriumAPI,
			raw:           nil,
			result: func(replicas *int32) bool {
				return replicas == &Replicas2
			},
		},
		{
			name:          "No replicas defined",
			componentName: config.ObservatoriumAPI,
			raw: &mcov1beta2.AdvancedConfig{
				ObservatoriumAPI: &mcov1beta2.CommonSpec{},
			},
			result: func(replicas *int32) bool {
				return replicas == &Replicas2
			},
		},
	}
	for _, c := range caseList {
		t.Run(c.componentName+":"+c.name, func(t *testing.T) {
			replicas := GetReplicas(c.componentName, c.raw)
			if !c.result(replicas) {
				t.Errorf("case (%v) output (%v) is not the expected", c.componentName+":"+c.name, replicas)
			}
		})
	}
}

func TestGetOperandName(t *testing.T) {
	caseList := []struct {
		name          string
		componentName string
		prepare       func()
		result        func() bool
	}{
		{
			name:          "No Observatorium CR",
			componentName: Alertmanager,
			prepare: func() {
				SetOperandNames(fake.NewFakeClientWithScheme(runtime.NewScheme()))
			},
			result: func() bool {
				return GetOperandName(Alertmanager) == config.GetOperandNamePrefix()+"alertmanager"
			},
		},
		{
			name:          "Have Observatorium CR without ownerreference",
			componentName: Alertmanager,
			prepare: func() {
				//clean the operandNames map
				CleanUpOperandNames()
				mco := &mcov1beta2.MultiClusterObservability{
					TypeMeta: metav1.TypeMeta{Kind: "MultiClusterObservability"},
					ObjectMeta: metav1.ObjectMeta{
						Name: config.GetDefaultCRName(),
					},
					Spec: mcov1beta2.MultiClusterObservabilitySpec{
						StorageConfig: &mcov1beta2.StorageConfig{
							MetricObjectStorage: &mcoshared.PreConfiguredStorage{
								Key:  "test",
								Name: "test",
							},
						},
					},
				}

				observatorium := &observatoriumv1alpha1.Observatorium{
					ObjectMeta: metav1.ObjectMeta{
						Name:      config.GetOperandNamePrefix() + "-observatorium",
						Namespace: config.GetDefaultNamespace(),
					},
				}

				// Register operator types with the runtime scheme.
				s := scheme.Scheme
				mcov1beta2.SchemeBuilder.AddToScheme(s)
				observatoriumv1alpha1.AddToScheme(s)
				client := fake.NewFakeClientWithScheme(s, []runtime.Object{mco, observatorium}...)
				config.SetMonitoringCRName(config.GetDefaultCRName())
				SetOperandNames(client)
			},
			result: func() bool {
				return GetOperandName(Alertmanager) == config.GetOperandNamePrefix()+Alertmanager &&
					GetOperandName(Grafana) == config.GetOperandNamePrefix()+Grafana &&
					GetOperandName(Observatorium) == config.GetDefaultCRName()
			},
		},
		{
			name:          "Have Observatorium CR (observability-observatorium) with ownerreference",
			componentName: Alertmanager,
			prepare: func() {
				//clean the operandNames map
				CleanUpOperandNames()
				mco := &mcov1beta2.MultiClusterObservability{
					TypeMeta: metav1.TypeMeta{Kind: "MultiClusterObservability"},
					ObjectMeta: metav1.ObjectMeta{
						Name: config.GetDefaultCRName(),
					},
					Spec: mcov1beta2.MultiClusterObservabilitySpec{
						StorageConfig: &mcov1beta2.StorageConfig{
							MetricObjectStorage: &mcoshared.PreConfiguredStorage{
								Key:  "test",
								Name: "test",
							},
						},
					},
				}

				observatorium := &observatoriumv1alpha1.Observatorium{
					ObjectMeta: metav1.ObjectMeta{
						Name:      config.GetOperandNamePrefix() + "observatorium",
						Namespace: config.GetDefaultNamespace(),
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "MultiClusterObservability",
								Name: config.GetDefaultCRName(),
							},
						},
					},
				}

				// Register operator types with the runtime scheme.
				s := scheme.Scheme
				mcov1beta2.SchemeBuilder.AddToScheme(s)
				observatoriumv1alpha1.AddToScheme(s)
				client := fake.NewFakeClientWithScheme(s, []runtime.Object{mco, observatorium}...)

				config.SetMonitoringCRName(config.GetDefaultCRName())
				SetOperandNames(client)
			},
			result: func() bool {
				return GetOperandName(Alertmanager) == Alertmanager &&
					GetOperandName(Grafana) == Grafana &&
					GetOperandName(Observatorium) == config.GetOperandNamePrefix()+"observatorium"
			},
		},
		{
			name:          "Have Observatorium CR (observability) with ownerreference",
			componentName: Alertmanager,
			prepare: func() {
				//clean the operandNames map
				CleanUpOperandNames()
				mco := &mcov1beta2.MultiClusterObservability{
					TypeMeta: metav1.TypeMeta{Kind: "MultiClusterObservability"},
					ObjectMeta: metav1.ObjectMeta{
						Name: config.GetDefaultCRName(),
					},
					Spec: mcov1beta2.MultiClusterObservabilitySpec{
						StorageConfig: &mcov1beta2.StorageConfig{
							MetricObjectStorage: &mcoshared.PreConfiguredStorage{
								Key:  "test",
								Name: "test",
							},
						},
					},
				}

				observatorium := &observatoriumv1alpha1.Observatorium{
					ObjectMeta: metav1.ObjectMeta{
						Name:      config.GetDefaultCRName(),
						Namespace: config.GetDefaultNamespace(),
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "MultiClusterObservability",
								Name: config.GetDefaultCRName(),
							},
						},
					},
				}

				// Register operator types with the runtime scheme.
				s := scheme.Scheme
				mcov1beta2.SchemeBuilder.AddToScheme(s)
				observatoriumv1alpha1.AddToScheme(s)
				client := fake.NewFakeClientWithScheme(s, []runtime.Object{mco, observatorium}...)

				config.SetMonitoringCRName(config.GetDefaultCRName())
				SetOperandNames(client)
			},
			result: func() bool {
				return GetOperandName(Alertmanager) == config.GetOperandNamePrefix()+Alertmanager &&
					GetOperandName(Grafana) == config.GetOperandNamePrefix()+Grafana &&
					GetOperandName(Observatorium) == config.GetDefaultCRName()
			},
		},
	}
	for _, c := range caseList {
		t.Run(c.name, func(t *testing.T) {
			c.prepare()
			if !c.result() {
				t.Errorf("case (%v) output is not the expected", c.name)
			}
		})
	}
}
