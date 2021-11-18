// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package observabilityagent

import (
	"context"
	"fmt"
	"reflect"
	"time"

	operatorv1 "github.com/openshift/api/operator/v1"
	ocpClientSet "github.com/openshift/client-go/config/clientset/versioned"
	oinformers "github.com/openshift/client-go/operator/informers/externalversions"
	operatorinformers "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/open-cluster-management/multicluster-observability-operator/operators/observabilityagent/pkg/config"
	mcoinformers "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/apis/multiclusterobservability/client/informers/externalversions"
	mcoinformer "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/apis/multiclusterobservability/client/informers/externalversions/multiclusterobservability/v1beta2"
	mcov1beta2 "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/apis/multiclusterobservability/v1beta2"
	obsv1beta2 "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/apis/multiclusterobservability/v1beta2"
	operatorsconfig "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/config"
)

// ObservabilityCoreController detects observability core resources are added,
// deleted, or updated
type ObservabilityCoreController struct {
	kubeClient client.Client
	ocpClient  ocpClientSet.Interface

	informerFactory            informers.SharedInformerFactory
	operatorInformerfactory    oinformers.SharedInformerFactory
	mcoOperatorInformerfactory mcoinformers.SharedInformerFactory

	configMapInformer      coreinformers.ConfigMapInformer
	secretInformer         coreinformers.SecretInformer
	serviceAccountInformer coreinformers.ServiceAccountInformer
	ingressInformer        operatorinformers.IngressControllerInformer
	mcoInformer            mcoinformer.MultiClusterObservabilityInformer
}

// Run starts shared informers and waits for the shared informer cache to
// synchronize.
func (o *ObservabilityCoreController) Start(ctx context.Context) error {
	// Starts all the shared informers that have been created by the factory so
	// far.
	o.informerFactory.Start(ctx.Done())
	o.operatorInformerfactory.Start(ctx.Done())
	o.mcoOperatorInformerfactory.Start(ctx.Done())
	// wait for the initial synchronization of the local cache.
	if !cache.WaitForCacheSync(ctx.Done(), o.configMapInformer.Informer().HasSynced,
		o.secretInformer.Informer().HasSynced, o.serviceAccountInformer.Informer().HasSynced,
		o.ingressInformer.Informer().HasSynced) {
		return fmt.Errorf("Failed to sync")
	}
	return nil
}

// ObservabilityCoreController creates a ObservabilityCoreController
func NewObservabilityCoreController(informerFactory informers.SharedInformerFactory,
	operatorInformerfactory oinformers.SharedInformerFactory,
	mcoOperatorInformerfactory mcoinformers.SharedInformerFactory,
	kubeClient client.Client) *ObservabilityCoreController {
	configMapInformer := informerFactory.Core().V1().ConfigMaps()
	secretInformer := informerFactory.Core().V1().Secrets()
	serviceAccountInformer := informerFactory.Core().V1().ServiceAccounts()
	ingressInformer := operatorInformerfactory.Operator().V1().IngressControllers()
	mcoInformer := mcoOperatorInformerfactory.Observability().V1beta2().MultiClusterObservabilities()

	o := &ObservabilityCoreController{
		informerFactory:        informerFactory,
		configMapInformer:      configMapInformer,
		secretInformer:         secretInformer,
		serviceAccountInformer: serviceAccountInformer,
		ingressInformer:        ingressInformer,
		mcoInformer:            mcoInformer,

		kubeClient: kubeClient,
	}
	configMapInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if obj.(*corev1.ConfigMap).GetName() == config.AllowlistCustomConfigMapName &&
					obj.(*corev1.ConfigMap).GetNamespace() == operatorsconfig.GetDefaultNamespace() {
					// generate the metrics allowlist configmap
					log.Info("generate metric allow list configmap for custom configmap CREATE")
					metricsAllowlistConfigMap, _ = generateMetricsListCM(o.kubeClient)
				}
			},
			UpdateFunc: func(old, new interface{}) {
				if new.(*corev1.ConfigMap).GetName() == config.AllowlistCustomConfigMapName &&
					new.(*corev1.ConfigMap).GetNamespace() == operatorsconfig.GetDefaultNamespace() &&
					new.(*corev1.ConfigMap).GetResourceVersion() != old.(*corev1.ConfigMap).GetResourceVersion() {
					// regenerate the metrics allowlist configmap
					log.Info("generate metric allow list configmap for custom configmap UPDATE")
					metricsAllowlistConfigMap, _ = generateMetricsListCM(o.kubeClient)
				}
			},
			DeleteFunc: func(obj interface{}) {
				if obj.(*corev1.ConfigMap).GetName() == config.AllowlistCustomConfigMapName &&
					obj.(*corev1.ConfigMap).GetNamespace() == operatorsconfig.GetDefaultNamespace() {
					// regenerate the metrics allowlist configmap
					log.Info("generate metric allow list configmap for custom configmap UPDATE")
					metricsAllowlistConfigMap, _ = generateMetricsListCM(o.kubeClient)
				}
			},
		},
	)

	secretInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if obj.(*corev1.Secret).GetName() == operatorsconfig.ServerCACerts &&
					obj.(*corev1.Secret).GetNamespace() == operatorsconfig.GetDefaultNamespace() {
					// generate the certificate for managed cluster
					log.Info("generate managedcluster observability certificate for server certificate CREATE")
					managedClusterObsCert, _ = generateObservabilityServerCACerts(o.kubeClient)
				}
			},
			UpdateFunc: func(old, new interface{}) {
				if (new.(*corev1.Secret).GetName() == operatorsconfig.ServerCACerts &&
					new.(*corev1.Secret).GetNamespace() == operatorsconfig.GetDefaultNamespace()) &&
					new.(*corev1.Secret).GetResourceVersion() != old.(*corev1.Secret).GetResourceVersion() {
					// regenerate the certificate for managed cluster
					log.Info("generate managedcluster observability certificate for server certificate UPDATE")
					managedClusterObsCert, _ = generateObservabilityServerCACerts(o.kubeClient)
				}
			},
			DeleteFunc: func(obj interface{}) {
			},
		},
	)

	serviceAccountInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if obj.(*corev1.ServiceAccount).GetName() == config.AlertmanagerAccessorSAName &&
					obj.(*corev1.ServiceAccount).GetNamespace() == operatorsconfig.GetDefaultNamespace() {
					// wait 10s for access_token of alertmanager and generate the secret that contains the access_token
					/* #nosec */
					wait.Poll(2*time.Second, 10*time.Second, func() (bool, error) {
						var err error
						log.Info("generate amAccessorTokenSecret for alertmanager access serviceaccount CREATE")
						if amAccessorTokenSecret, err = generateAmAccessorTokenSecret(o.kubeClient); err == nil {
							return true, nil
						}
						return false, err
					})
				}
			},
			UpdateFunc: func(old, new interface{}) {
				if (new.(*corev1.ServiceAccount).GetName() == config.AlertmanagerAccessorSAName &&
					new.(*corev1.ServiceAccount).GetNamespace() == operatorsconfig.GetDefaultNamespace()) &&
					new.(*corev1.ServiceAccount).GetResourceVersion() !=
						old.(*corev1.ServiceAccount).GetResourceVersion() {
					// regenerate the secret that contains the access_token for the Alertmanager in the Hub cluster
					amAccessorTokenSecret, _ = generateAmAccessorTokenSecret(o.kubeClient)
				}
			},
			DeleteFunc: func(obj interface{}) {
			},
		},
	)

	ingressInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if obj.(*operatorv1.IngressController).GetName() == operatorsconfig.OpenshiftIngressOperatorCRName &&
					obj.(*operatorv1.IngressController).GetNamespace() == operatorsconfig.OpenshiftIngressOperatorNamespace {
					// generate the hubInfo secret
					hubInfoSecret, _ = generateHubInfoSecret(o.kubeClient, operatorsconfig.GetDefaultNamespace(), spokeNameSpace, true)
				}
			},
			UpdateFunc: func(old, new interface{}) {
				if new.(*operatorv1.IngressController).GetName() == operatorsconfig.OpenshiftIngressOperatorCRName &&
					new.(*operatorv1.IngressController).GetResourceVersion() != old.(*operatorv1.IngressController).GetResourceVersion() &&
					new.(*operatorv1.IngressController).GetNamespace() == operatorsconfig.OpenshiftIngressOperatorNamespace {
					// regenerate the hubInfo secret
					hubInfoSecret, _ = generateHubInfoSecret(o.kubeClient, operatorsconfig.GetDefaultNamespace(), spokeNameSpace, true)
				}
			},
			DeleteFunc: func(obj interface{}) {
				if obj.(*operatorv1.IngressController).GetName() == operatorsconfig.OpenshiftIngressOperatorCRName &&
					obj.(*operatorv1.IngressController).GetNamespace() == operatorsconfig.OpenshiftIngressOperatorNamespace {
					// regenerate the hubInfo secret
					hubInfoSecret, _ = generateHubInfoSecret(o.kubeClient, operatorsconfig.GetDefaultNamespace(), spokeNameSpace, true)
				}
			},
			//TODO: ingressCtlCrdExists to replace true
			// send request to trigger reconciler
			// check amRouterCertSecretPred and routeCASecretPred secret
		},
	)

	o.mcoInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {

			operatorsconfig.SetMonitoringCRName(obj.(*obsv1beta2.MultiClusterObservability).GetName())
			// generate the image pull secret
			pullSecret, _ = generatePullSecret(o.kubeClient, operatorsconfig.GetImagePullSecret(obj.(*obsv1beta2.MultiClusterObservability).Spec))
		},
		UpdateFunc: func(old, new interface{}) {
			// only reconcile when ObservabilityAddonSpec updated
			if new.(*obsv1beta2.MultiClusterObservability).GetResourceVersion() !=
				old.(*obsv1beta2.MultiClusterObservability).GetResourceVersion() &&
				!reflect.DeepEqual(new.(*mcov1beta2.MultiClusterObservability).Spec.ObservabilityAddonSpec,
					old.(*mcov1beta2.MultiClusterObservability).Spec.ObservabilityAddonSpec) {
				if new.(*mcov1beta2.MultiClusterObservability).Spec.ImagePullSecret != old.(*mcov1beta2.MultiClusterObservability).Spec.ImagePullSecret {
					// regenerate the image pull secret
					pullSecret, _ = generatePullSecret(o.kubeClient, operatorsconfig.GetImagePullSecret(new.(*mcov1beta2.MultiClusterObservability).Spec))
				}
			}
		},
		DeleteFunc: func(obj interface{}) {},
	})

	return o
}
