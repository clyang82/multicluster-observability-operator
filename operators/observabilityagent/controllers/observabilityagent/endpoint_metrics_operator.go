// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package observabilityagent

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/kustomize/v3/pkg/resource"

	mcov1beta2 "github.com/open-cluster-management/multicluster-observability-operator/operators/multiclusterobservability/api/v1beta2"
	"github.com/open-cluster-management/multicluster-observability-operator/operators/observabilityagent/pkg/config"
	"github.com/open-cluster-management/multicluster-observability-operator/operators/observabilityagent/pkg/rendering/templates"
	operatorsconfig "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/config"
	operatorstemplates "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/rendering/templates"
	"github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/util"
)

const (
	deployName      = "endpoint-observability-operator"
	saName          = "endpoint-observability-operator-sa"
	rolebindingName = "open-cluster-management:endpoint-observability-operator-rb"
)

// loadTemplates load manifests from manifests directory
func loadTemplates(mco *mcov1beta2.MultiClusterObservability) (
	[]runtime.RawExtension,
	*apiextensionsv1.CustomResourceDefinition,
	*apiextensionsv1beta1.CustomResourceDefinition,
	*appsv1.Deployment,
	*corev1.ConfigMap,
	error) {
	// render endpoint-observability templates
	endpointObsTemplates, err := templates.GetOrLoadEndpointObservabilityTemplates(operatorstemplates.GetTemplateRenderer())
	if err != nil {
		log.Error(err, "Failed to load templates")
		return nil, nil, nil, nil, nil, err
	}
	crdv1 := &apiextensionsv1.CustomResourceDefinition{}
	crdv1beta1 := &apiextensionsv1beta1.CustomResourceDefinition{}
	dep := &appsv1.Deployment{}
	imageListCM := &corev1.ConfigMap{}
	rawExtensionList := []runtime.RawExtension{}
	for _, r := range endpointObsTemplates {
		obj, err := updateRes(r, mco)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		if r.GetKind() == "Deployment" {
			dep = obj.(*appsv1.Deployment)
		} else if r.GetKind() == "ConfigMap" && r.GetName() == operatorsconfig.ImageConfigMap {
			imageListCM = obj.(*corev1.ConfigMap)
		} else if r.GetKind() == "CustomResourceDefinition" {
			if r.GetGvk().Version == "v1" {
				crdv1 = obj.(*apiextensionsv1.CustomResourceDefinition)
			} else {
				crdv1beta1 = obj.(*apiextensionsv1beta1.CustomResourceDefinition)
			}
		} else {
			rawExtensionList = append(rawExtensionList, runtime.RawExtension{Object: obj})
		}
	}
	return rawExtensionList, crdv1, crdv1beta1, dep, imageListCM, nil
}

func updateRes(r *resource.Resource,
	mco *mcov1beta2.MultiClusterObservability) (runtime.Object, error) {

	kind := r.GetKind()
	if kind != "ClusterRole" && kind != "ClusterRoleBinding" && kind != "CustomResourceDefinition" {
		r.SetNamespace(spokeNameSpace)
	}
	obj := util.GetK8sObj(kind)
	if kind == "CustomResourceDefinition" && r.GetGvk().Version == "v1beta1" {
		obj = &apiextensionsv1beta1.CustomResourceDefinition{}
	}
	obj.GetObjectKind()
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(r.Map(), obj)
	if err != nil {
		log.Error(err, "failed to convert the resource", "resource", r.GetName())
		return nil, err
	}
	// set the images and watch_namespace for endpoint metrics operator
	if r.GetKind() == "Deployment" && r.GetName() == deployName {
		spec := obj.(*appsv1.Deployment).Spec.Template.Spec
		for i, container := range spec.Containers {
			if container.Name == "endpoint-observability-operator" {
				spec.Containers[i] = updateEndpointOperator(mco, container)
			}
		}
	}
	// set the imagepullsecrets for sa
	if r.GetKind() == "ServiceAccount" && r.GetName() == saName {
		imageSecrets := obj.(*corev1.ServiceAccount).ImagePullSecrets
		for i, imageSecret := range imageSecrets {
			if imageSecret.Name == "REPLACE_WITH_IMAGEPULLSECRET" {
				imageSecrets[i].Name = operatorsconfig.GetImagePullSecret(mco.Spec)
				break
			}
		}
	}
	// set namespace for rolebinding
	if r.GetKind() == "ClusterRoleBinding" && r.GetName() == rolebindingName {
		binding := obj.(*rbacv1.ClusterRoleBinding)
		binding.Subjects[0].Namespace = spokeNameSpace
	}
	// set images for components in managed clusters
	if r.GetKind() == "ConfigMap" && r.GetName() == operatorsconfig.ImageConfigMap {
		images := obj.(*corev1.ConfigMap).Data
		for key, _ := range images {
			if key == operatorsconfig.ConfigmapReloaderKey {
				found, image := operatorsconfig.ReplaceImage(
					mco.Annotations,
					operatorsconfig.ConfigmapReloaderImgRepo+"/"+operatorsconfig.ImageKeyNameMap[operatorsconfig.ConfigmapReloaderKey],
					key)
				if found {
					obj.(*corev1.ConfigMap).Data[key] = image
				}
			} else {
				found, image := operatorsconfig.ReplaceImage(
					mco.Annotations,
					operatorsconfig.DefaultImgRepository+"/"+operatorsconfig.ImageKeyNameMap[key],
					key)
				if found {
					obj.(*corev1.ConfigMap).Data[key] = image
				}
			}
		}
	}

	return obj, nil
}

func updateEndpointOperator(mco *mcov1beta2.MultiClusterObservability,
	container corev1.Container) corev1.Container {
	container.Image = getImage(mco, config.EndpointControllerImgName,
		operatorsconfig.DefaultImgTagSuffix, config.EndpointControllerKey)
	container.ImagePullPolicy = operatorsconfig.GetImagePullPolicy(mco.Spec)
	for i, env := range container.Env {
		if env.Name == operatorsconfig.PullSecret {
			container.Env[i].Value = operatorsconfig.GetImagePullSecret(mco.Spec)
		}
	}
	return container
}

func getImage(mco *mcov1beta2.MultiClusterObservability,
	name, tag, key string) string {
	image := operatorsconfig.DefaultImgRepository +
		"/" + name + ":" + tag
	found, replacedImage := operatorsconfig.ReplaceImage(mco.Annotations, image, key)
	if found {
		return replacedImage
	}
	return image
}

func loadPromTemplates(mco *mcov1beta2.MultiClusterObservability) (
	[]runtime.RawExtension, error) {
	// load and render promTemplates
	promTemplates, err := templates.GetOrLoadPrometheusTemplates(operatorstemplates.GetTemplateRenderer())
	if err != nil {
		log.Error(err, "Failed to load templates")
		return nil, err
	}
	rawExtensionList := []runtime.RawExtension{}
	for _, r := range promTemplates {
		obj, err := updateRes(r, mco)
		if err != nil {
			return nil, err
		}
		rawExtensionList = append(rawExtensionList, runtime.RawExtension{Object: obj})
	}
	return rawExtensionList, nil
}
