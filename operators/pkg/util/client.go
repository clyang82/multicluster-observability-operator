// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package util

import (
	"context"
	"fmt"

	ocpClientSet "github.com/openshift/client-go/config/clientset/versioned"
	ocpOperatorClientset "github.com/openshift/client-go/operator/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	crdClientSet "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mcoClientset "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/apis/multiclusterobservability/client/clientset/versioned"
	"github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/config"
)

var (
	crdClient crdClientSet.Interface
	ocpClient ocpClientSet.Interface
)

// CreateKubeClient creates new kube client
func CreateKubeClient(kubeconfigPath string, schema *runtime.Scheme) (client.Client, error) {
	// create the config from the path
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		log.Error(err, "Failed to create the config")
		return nil, err
	}

	// generate the client based off of the config
	kubeClient, err := client.New(config, client.Options{Scheme: schema})
	if err != nil {
		log.Error(err, "Failed to create kube client")
		return nil, err
	}

	return kubeClient, nil
}

// CreateKubeClientset creates new kubernetes clientset
func CreateKubeClientset(kubeconfigPath string) (*kubernetes.Clientset, error) {
	// create the config from the path
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		log.Error(err, "Failed to create the config")
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error(err, "Failed to create kube client")
		return nil, err
	}

	return clientset, nil
}

// CreateOCPClient creates ocp client
func CreateOCPClient(kubeconfigPath string) (ocpClientSet.Interface, error) {
	// create the config from the path
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		log.Error(err, "Failed to create the config")
		return nil, err
	}

	// generate the client based off of the config
	ocpClient, err = ocpClientSet.NewForConfig(config)
	if err != nil {
		log.Error(err, "Failed to create ocp config client")
		return nil, err
	}

	return ocpClient, err
}

func CreateOCPOperatorClientset(kubeconfigPath string) (*ocpOperatorClientset.Clientset, error) {
	// create the config from the path
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		log.Error(err, "Failed to create the config")
		return nil, err
	}

	// generate the client based off of the config
	oClientset, err := ocpOperatorClientset.NewForConfig(config)
	if err != nil {
		log.Error(err, "Failed to create ocp config client")
		return nil, err
	}

	return oClientset, err
}

func CreateMCOClientset(kubeconfigPath string) (*mcoClientset.Clientset, error) {
	// create the config from the path
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		log.Error(err, "Failed to create the config")
		return nil, err
	}

	// generate the client based off of the config
	mcoClientset, err := mcoClientset.NewForConfig(config)
	if err != nil {
		log.Error(err, "Failed to create ocp config client")
		return nil, err
	}

	return mcoClientset, err
}

// GetOrCreateCRDClient gets an existing or creates a new CRD client
func GetOrCreateCRDClient() (crdClientSet.Interface, error) {
	if crdClient != nil {
		return crdClient, nil
	}
	// create the config from the path
	config, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		log.Error(err, "Failed to create the config")
		return nil, err
	}

	// generate the client based off of the config
	crdClient, err = crdClientSet.NewForConfig(config)
	if err != nil {
		log.Error(err, "Failed to create CRD config client")
		return nil, err
	}

	return crdClient, err
}

func CheckCRDExist(crdClient crdClientSet.Interface, crdName string) (bool, error) {
	_, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), crdName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("unable to get CRD with ApiextensionsV1 Client, not found.", "CRD", crdName)
			return false, nil
		}
		log.Error(err, "failed to get CRD with ApiextensionsV1 Client", "CRD", crdName)
		return false, err
	}
	return true, nil
}

func UpdateCRDWebhookNS(crdClient crdClientSet.Interface, namespace, crdName string) error {
	crdObj, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), crdName, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "failed to get CRD", "CRD", crdName)
		return err
	}
	if crdObj.Spec.Conversion == nil || crdObj.Spec.Conversion.Webhook == nil || crdObj.Spec.Conversion.Webhook.ClientConfig == nil {
		log.Error(err, "empty Conversion in the CRD", "CRD", crdName)
		return fmt.Errorf("empty Conversion in the CRD %s", crdName)
	}
	if crdObj.Spec.Conversion.Webhook.ClientConfig.Service.Namespace != namespace {
		log.Info("updating the webhook service namespace", "original namespace", crdObj.Spec.Conversion.Webhook.ClientConfig.Service.Namespace, "new namespace", namespace)
		crdObj.Spec.Conversion.Webhook.ClientConfig.Service.Namespace = namespace
		_, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Update(context.TODO(), crdObj, metav1.UpdateOptions{})
		if err != nil {
			log.Error(err, "failed to update webhook service namespace")
			return err
		}
	}
	return nil
}

// GetPVCList get pvc with matched labels
func GetPVCList(c client.Client, matchLabels map[string]string) ([]corev1.PersistentVolumeClaim, error) {
	pvcList := &corev1.PersistentVolumeClaimList{}
	pvcListOpts := []client.ListOption{
		client.InNamespace(config.GetDefaultNamespace()),
		client.MatchingLabels(matchLabels),
	}

	err := c.List(context.TODO(), pvcList, pvcListOpts...)
	if err != nil {
		return nil, err
	}
	return pvcList.Items, nil
}

// GetStatefulSetList get sts with matched labels
func GetStatefulSetList(c client.Client, matchLabels map[string]string) ([]appsv1.StatefulSet, error) {
	stsList := &appsv1.StatefulSetList{}
	stsListOpts := []client.ListOption{
		client.InNamespace(config.GetDefaultNamespace()),
		client.MatchingLabels(matchLabels),
	}

	err := c.List(context.TODO(), stsList, stsListOpts...)
	if err != nil {
		return nil, err
	}
	return stsList.Items, nil
}
