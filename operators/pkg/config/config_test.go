// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package config

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	observatoriumv1alpha1 "github.com/open-cluster-management/observatorium-operator/api/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mcov1beta2 "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/apis/multiclusterobservability/v1beta2"
)

var (
	apiServerURL           = "http://example.com"
	clusterID              = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
	version                = "2.1.1"
	DefaultDSImgRepository = "quay.io:443/acm-d"
)

func TestReplaceImage(t *testing.T) {

	caseList := []struct {
		annotations map[string]string
		name        string
		imageRepo   string
		expected    bool
		cm          map[string]string
	}{
		{
			annotations: map[string]string{
				AnnotationKeyImageRepository: DefaultImgRepository,
				"mco-test-image":             "test.org/test:latest",
			},
			name:      "Replace image for test purpose",
			imageRepo: "test.org",
			expected:  true,
			cm:        nil,
		},

		{
			annotations: map[string]string{
				AnnotationKeyImageRepository: DefaultImgRepository,
				AnnotationKeyImageTagSuffix:  "test",
			},
			name:      "Image is in different org",
			imageRepo: "test.org",
			expected:  false,
			cm:        nil,
		},

		{
			annotations: map[string]string{
				AnnotationKeyImageRepository: DefaultImgRepository,
			},
			name:      "Image is in different org",
			imageRepo: "test.org",
			expected:  false,
			cm:        nil,
		},

		{
			annotations: map[string]string{
				AnnotationKeyImageRepository: DefaultImgRepository,
				AnnotationKeyImageTagSuffix:  "2.3.0-SNAPSHOT-2021-07-26-18-43-26",
			},
			name:      "Image is in the same org",
			imageRepo: DefaultImgRepository,
			expected:  true,
			cm:        nil,
		},

		{
			annotations: map[string]string{
				AnnotationKeyImageRepository: DefaultImgRepository,
			},
			name:      "Image is in the same org",
			imageRepo: DefaultImgRepository,
			expected:  false,
			cm:        nil,
		},

		{
			annotations: map[string]string{
				AnnotationKeyImageRepository: DefaultImgRepository,
			},
			name:      "Image is in the same org",
			imageRepo: DefaultImgRepository,
			expected:  true,
			cm: map[string]string{
				"test": "test.org",
			},
		},

		{
			annotations: map[string]string{
				AnnotationKeyImageRepository: DefaultDSImgRepository,
				AnnotationKeyImageTagSuffix:  "2.3.0-SNAPSHOT-2021-07-26-18-43-26",
			},
			name:      "Image is from the ds build",
			imageRepo: "test.org",
			expected:  false,
			cm:        nil,
		},

		{
			annotations: map[string]string{
				AnnotationKeyImageRepository: DefaultDSImgRepository,
			},
			name:      "Image is from the ds build",
			imageRepo: "test.org",
			expected:  true,
			cm: map[string]string{
				"test": "test.org",
			},
		},

		{
			annotations: map[string]string{
				AnnotationKeyImageRepository: DefaultDSImgRepository,
			},
			name:      "Image is from the ds build",
			imageRepo: "test.org",
			expected:  false,
			cm:        nil,
		},

		{
			annotations: map[string]string{
				AnnotationKeyImageRepository: "",
				AnnotationKeyImageTagSuffix:  "",
			},
			name:      "the img repo is empty",
			imageRepo: "",
			expected:  false,
			cm:        nil,
		},

		{
			annotations: map[string]string{},
			name:        "no img info",
			imageRepo:   "test.org",
			expected:    false,
			cm:          nil,
		},

		{
			annotations: nil,
			name:        "annotations is nil",
			imageRepo:   "test.org",
			expected:    false,
			cm:          nil,
		},
	}

	for _, c := range caseList {
		t.Run(c.name, func(t *testing.T) {
			SetImageManifests(c.cm)
			output, _ := ReplaceImage(c.annotations, c.imageRepo, "test")
			if output != c.expected {
				t.Errorf("case (%v) output (%v) is not the expected (%v)", c.name, output, c.expected)
			}
		})
	}
}

func TestGetDefaultNamespace(t *testing.T) {
	expected := "open-cluster-management-observability"
	if GetDefaultNamespace() != expected {
		t.Errorf("Default Namespace (%v) is not the expected (%v)", GetDefaultNamespace(), expected)
	}
}

func TestMonitoringCRName(t *testing.T) {
	var monitoringCR = "monitoring"
	SetMonitoringCRName(monitoringCR)

	if monitoringCR != GetMonitoringCRName() {
		t.Errorf("Monitoring CR Name (%v) is not the expected (%v)", GetMonitoringCRName(), monitoringCR)
	}
}

func TestIsPaused(t *testing.T) {
	caseList := []struct {
		annotations map[string]string
		expected    bool
		name        string
	}{
		{
			name: "without mco-pause",
			annotations: map[string]string{
				AnnotationKeyImageRepository: DefaultImgRepository,
				AnnotationKeyImageTagSuffix:  "test",
			},
			expected: false,
		},
		{
			name: "mco-pause is empty",
			annotations: map[string]string{
				AnnotationMCOPause: "",
			},
			expected: false,
		},
		{
			name: "mco-pause is false",
			annotations: map[string]string{
				AnnotationMCOPause: "false",
			},
			expected: false,
		},
		{
			name: "mco-pause is true",
			annotations: map[string]string{
				AnnotationMCOPause: "true",
			},
			expected: true,
		},
	}

	for _, c := range caseList {
		t.Run(c.name, func(t *testing.T) {
			output := IsPaused(c.annotations)
			if output != c.expected {
				t.Errorf("case (%v) output (%v) is not the expected (%v)", c.name, output, c.expected)
			}
		})
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

func TestReadImageManifestConfigMap(t *testing.T) {
	var buildTestImageManifestCM func(ns, version string) *corev1.ConfigMap
	buildTestImageManifestCM = func(ns, version string) *corev1.ConfigMap {
		return &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ImageManifestConfigMapNamePrefix + version,
				Namespace: ns,
				Labels: map[string]string{
					OCMManifestConfigMapTypeLabelKey:    OCMManifestConfigMapTypeLabelValue,
					OCMManifestConfigMapVersionLabelKey: version,
				},
			},
			Data: map[string]string{
				"test-key": fmt.Sprintf("test-value:%s", version),
			},
		}
	}

	ns := "testing"
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)

	caseList := []struct {
		name         string
		inputCMList  []string
		version      string
		expectedData map[string]string
		expectedRet  bool
		preFunc      func()
	}{
		{
			name:         "no image manifest configmap",
			inputCMList:  []string{},
			version:      "2.3.0",
			expectedRet:  false,
			expectedData: map[string]string{},
			preFunc: func() {
				os.Setenv("POD_NAMESPACE", ns)
				SetImageManifests(map[string]string{})
			},
		},
		{
			name:         "single valid image manifest configmap",
			inputCMList:  []string{"2.2.3"},
			version:      "2.3.0",
			expectedRet:  false,
			expectedData: map[string]string{},
			preFunc: func() {
				os.Setenv("POD_NAMESPACE", ns)
				SetImageManifests(map[string]string{})
			},
		},
		{
			name:        "multiple valid image manifest configmaps",
			inputCMList: []string{"2.2.3", "2.3.0"},
			version:     "2.3.0",
			expectedRet: true,
			expectedData: map[string]string{
				"test-key": "test-value:2.3.0",
			},
			preFunc: func() {
				os.Setenv("POD_NAMESPACE", ns)
				SetImageManifests(map[string]string{})
			},
		},
		{
			name:        "multiple image manifest configmaps with invalid",
			inputCMList: []string{"2.2.3", "2.3.0", "invalid"},
			version:     "2.3.0",
			expectedRet: true,
			expectedData: map[string]string{
				"test-key": "test-value:2.3.0",
			},
			preFunc: func() {
				os.Setenv("POD_NAMESPACE", ns)
				SetImageManifests(map[string]string{})
			},
		},
		{
			name:         "valid image manifest configmaps with no namespace set",
			inputCMList:  []string{"2.2.3", "2.3.0"},
			version:      "2.3.0",
			expectedRet:  false,
			expectedData: map[string]string{},
			preFunc: func() {
				os.Unsetenv("POD_NAMESPACE")
				SetImageManifests(map[string]string{})
			},
		},
	}

	for _, c := range caseList {
		t.Run(c.name, func(t *testing.T) {
			c.preFunc()
			initObjs := []runtime.Object{}
			for _, cmName := range c.inputCMList {
				initObjs = append(initObjs, buildTestImageManifestCM(ns, cmName))
			}
			client := fake.NewFakeClientWithScheme(scheme, initObjs...)

			gotRet, err := ReadImageManifestConfigMap(client, c.version)
			if err != nil {
				t.Errorf("Failed read image manifest configmap due to %v", err)
			}
			if gotRet != c.expectedRet {
				t.Errorf("case (%v) output (%v) is not the expected (%v)", c.name, gotRet, c.expectedRet)
			}
			if !reflect.DeepEqual(GetImageManifests(), c.expectedData) {
				t.Errorf("case (%v) output (%v) is not the expected (%v)", c.name, GetImageManifests(), c.expectedData)
			}
		})
	}
}

func TestGetObsAPIHost(t *testing.T) {
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      obsAPIGateway,
			Namespace: "test",
		},
		Spec: routev1.RouteSpec{
			Host: apiServerURL,
		},
	}
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(routev1.GroupVersion, route)
	client := fake.NewFakeClientWithScheme(scheme, route)

	host, _ := GetObsAPIHost(client, "default")
	if host == apiServerURL {
		t.Errorf("Should not get route host in default namespace")
	}
	host, _ = GetObsAPIHost(client, "test")
	if host != apiServerURL {
		t.Errorf("Observatorium api (%v) is not the expected (%v)", host, apiServerURL)
	}
}
