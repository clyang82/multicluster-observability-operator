// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package observabilityagent

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	operatorclientset "github.com/openshift/client-go/operator/clientset/versioned"
	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	certctrl "github.com/open-cluster-management/multicluster-observability-operator/operators/observabilityagent/pkg/certificates"
	"github.com/open-cluster-management/multicluster-observability-operator/operators/observabilityagent/pkg/util"
	mcov1beta1 "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/apis/multiclusterobservability/v1beta1"
	mcov1beta2 "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/apis/multiclusterobservability/v1beta2"
	operatorsconfig "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/config"
	operatorutil "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/util"
	mchv1 "github.com/open-cluster-management/multiclusterhub-operator/pkg/apis/operator/v1"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
)

const (
	ownerLabelKey             = "owner"
	ownerLabelValue           = "multicluster-observability-operator"
	managedClusterObsCertName = "observability-managed-cluster-certs"
	nonOCP                    = "N/A"
)

var (
	log                             = logf.Log.WithName("controller_observabilityagent")
	watchNamespace                  = operatorsconfig.GetDefaultNamespace()
	isCRoleCreated                  = false
	isClusterManagementAddonCreated = false
	isplacementControllerRunnning   = false
	managedClusterList              = map[string]string{}
)

// ObservabilityAgentReconciler
type ObservabilityAgentReconciler struct {
	Client            client.Client
	Log               logr.Logger
	Scheme            *runtime.Scheme
	CRDMap            map[string]bool
	RESTMapper        meta.RESTMapper
	KubeClient        client.Client
	RouteClientset    routeclientset.Clientset
	OperatorClientset operatorclientset.Clientset
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ObservabilityAgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling ObservabilityAgent")

	if operatorsconfig.GetMonitoringCRName() == "" {
		reqLogger.Info("multicluster observability resource is not available")
		return ctrl.Result{}, nil
	}

	// setup ocm addon manager
	certctrl.Start()

	deleteAll := false
	// Fetch the MultiClusterObservability instance
	mco := &mcov1beta2.MultiClusterObservability{}
	err := r.KubeClient.Get(context.TODO(),
		types.NamespacedName{
			Name: operatorsconfig.GetMonitoringCRName(),
		}, mco)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			deleteAll = true
		} else {
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}
	}

	// Do not reconcile objects if this instance of mch is labeled "paused"
	if operatorsconfig.IsPaused(mco.GetAnnotations()) {
		reqLogger.Info("MCO reconciliation is paused. Nothing more to do.")
		return ctrl.Result{}, nil
	}

	// check if the MCH CRD exists
	mchCrdExists, _ := r.CRDMap[operatorsconfig.MCHCrdName]
	// requeue after 10 seconds if the mch crd exists and image image manifests map is empty
	if mchCrdExists && len(operatorsconfig.GetImageManifests()) == 0 {
		// if the mch CR is not ready, then requeue the request after 10s
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// check if the server certificate for managedcluster
	if managedClusterObsCert == nil {
		var err error
		managedClusterObsCert, err = generateObservabilityServerCACerts(r.KubeClient)
		if err != nil && k8serrors.IsNotFound(err) {
			// if the servser certificate for managedcluster is not ready, then requeue the request after 10s to avoid useless reconcile loop.
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	opts := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{ownerLabelKey: ownerLabelValue}),
	}
	if req.Namespace != operatorsconfig.GetDefaultNamespace() &&
		req.Namespace != "" {
		opts.Namespace = req.Namespace
	}

	obsAddonList := &mcov1beta1.ObservabilityAddonList{}
	err = r.Client.List(context.TODO(), obsAddonList, opts)
	if err != nil {
		reqLogger.Error(err, "Failed to list observabilityaddon resource")
		return ctrl.Result{}, err
	}

	if !deleteAll {
		res, err := createAllRelatedRes(r.Client, r.KubeClient, r.RouteClientset, r.OperatorClientset, r.RESTMapper,
			req, mco, obsAddonList, true)
		//TODO: r.CRDMap[operatorsconfig.IngressControllerCRD] should be from obs core
		if err != nil {
			return res, err
		}
	} else {
		res, err := deleteAllObsAddons(r.Client, obsAddonList)
		if err != nil {
			return res, err
		}
	}

	obsAddonList = &mcov1beta1.ObservabilityAddonList{}
	err = r.Client.List(context.TODO(), obsAddonList, opts)
	if err != nil {
		reqLogger.Error(err, "Failed to list observabilityaddon resource")
		return ctrl.Result{}, err
	}
	workList := &workv1.ManifestWorkList{}
	err = r.Client.List(context.TODO(), workList, opts)
	if err != nil {
		reqLogger.Error(err, "Failed to list manifestwork resource")
		return ctrl.Result{}, err
	}
	managedclusteraddonList := &addonv1alpha1.ManagedClusterAddOnList{}
	err = r.Client.List(context.TODO(), managedclusteraddonList, opts)
	if err != nil {
		reqLogger.Error(err, "Failed to list managedclusteraddon resource")
		return ctrl.Result{}, err
	}
	latestClusters := []string{}
	staleAddons := []string{}
	for _, addon := range obsAddonList.Items {
		latestClusters = append(latestClusters, addon.Namespace)
		staleAddons = append(staleAddons, addon.Namespace)
	}
	for _, work := range workList.Items {
		if work.Name != work.Namespace+workNameSuffix {
			reqLogger.Info("To delete invalid manifestwork", "name", work.Name, "namespace", work.Namespace)
			err = deleteManifestWork(r.Client, work.Name, work.Namespace)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		if !operatorutil.Contains(latestClusters, work.Namespace) {
			reqLogger.Info("To delete manifestwork", "namespace", work.Namespace)
			err = deleteManagedClusterRes(r.Client, work.Namespace)
			if err != nil {
				return ctrl.Result{}, err
			}
		} else {
			staleAddons = operatorutil.Remove(staleAddons, work.Namespace)
		}
	}

	// after the managedcluster is detached, the manifestwork for observability will be delete be the cluster manager,
	// but the managedclusteraddon for observability will not deleted by the cluster manager, so check against the
	// managedclusteraddon list to remove the managedcluster resources after the managedcluster is detached.
	for _, mcaddon := range managedclusteraddonList.Items {
		if !operatorutil.Contains(latestClusters, mcaddon.Namespace) {
			reqLogger.Info("To delete managedcluster resources", "namespace", mcaddon.Namespace)
			err = deleteManagedClusterRes(r.Client, mcaddon.Namespace)
			if err != nil {
				return ctrl.Result{}, err
			}
		} else {
			staleAddons = operatorutil.Remove(staleAddons, mcaddon.Namespace)
		}
	}

	// delete stale addons if manifestwork does not exist
	for _, addon := range staleAddons {
		err = deleteStaleObsAddon(r.Client, addon, true)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// only update managedclusteraddon status when obs addon's status updated
	if req.Name == obsAddonName {
		err = updateAddonStatus(r.Client, *obsAddonList)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if deleteAll {
		opts.Namespace = ""
		err = r.Client.List(context.TODO(), workList, opts)
		if err != nil {
			reqLogger.Error(err, "Failed to list manifestwork resource")
			return ctrl.Result{}, err
		}
		if len(workList.Items) == 0 {
			err = deleteGlobalResource(r.Client)
		}
	}

	return ctrl.Result{}, err
}

func createAllRelatedRes(
	c client.Client,
	kubeClient client.Client,
	routeClientset routeclientset.Clientset,
	operatorClientset operatorclientset.Clientset,
	restMapper meta.RESTMapper,
	request ctrl.Request,
	mco *mcov1beta2.MultiClusterObservability,
	obsAddonList *mcov1beta1.ObservabilityAddonList,
	ingressCtlCrdExists bool) (ctrl.Result, error) {

	// create the clusterrole if not there
	if !isCRoleCreated {
		err := createClusterRole(c)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = createResourceRole(c)
		if err != nil {
			return ctrl.Result{}, err
		}
		isCRoleCreated = true
	}
	//Check if ClusterManagementAddon is created or create it
	if !isClusterManagementAddonCreated {
		err := util.CreateClusterManagementAddon(c)
		if err != nil {
			return ctrl.Result{}, err
		}
		isClusterManagementAddonCreated = true
	}

	currentClusters := []string{}
	for _, ep := range obsAddonList.Items {
		currentClusters = append(currentClusters, ep.Namespace)
	}

	// need to reload the template and update the the corresponding resources
	// the loadTemplates method is now lightweight operations as we have cache the templates in memory.
	log.Info("load and update templates for managedcluster resources")
	rawExtensionList, obsAddonCRDv1, obsAddonCRDv1beta1,
		endpointMetricsOperatorDeploy, imageListConfigMap, _ = loadTemplates(mco)

	works, crdv1Work, crdv1beta1Work, err := generateGlobalManifestResources(kubeClient, mco)
	if err != nil {
		return ctrl.Result{}, err
	}

	// regenerate the hubinfo secret if empty
	if hubInfoSecret == nil {
		var err error
		if hubInfoSecret, err = generateHubInfoSecret(kubeClient, routeClientset, operatorClientset,
			operatorsconfig.GetDefaultNamespace(), spokeNameSpace, ingressCtlCrdExists); err != nil {
			return ctrl.Result{}, err
		}
	}

	failedCreateManagedClusterRes := false
	for managedCluster, openshiftVersion := range managedClusterList {
		currentClusters = operatorutil.Remove(currentClusters, managedCluster)
		// enter the loop for the following reconcile requests:
		// 1. MCO CR change(request name is "mco-updated-request")
		// 2. MCH resource change(request name is "mch-updated-request"), to handle image replacement in upgrade case.
		// 3. configmap/secret... resource change from observability namespace
		// 4. managedcluster change(request namespace is emprt string and request name is managedcluster name)
		// 5. manifestwork/observabilityaddon/managedclusteraddon/rolebinding... change from managedcluster namespace
		if request.Name == operatorsconfig.MCOUpdatedRequestName ||
			request.Name == operatorsconfig.MCHUpdatedRequestName ||
			request.Namespace == operatorsconfig.GetDefaultNamespace() ||
			(request.Namespace == "" && request.Name == managedCluster) ||
			request.Namespace == managedCluster {
			log.Info("Monitoring operator should be installed in cluster", "cluster_name", managedCluster, "request.name", request.Name, "request.namespace", request.Namespace)
			if openshiftVersion == "3" {
				err = createManagedClusterRes(c, restMapper, mco,
					managedCluster, managedCluster,
					works, crdv1beta1Work, endpointMetricsOperatorDeploy, hubInfoSecret, false)
			} else if openshiftVersion == nonOCP {
				err = createManagedClusterRes(c, restMapper, mco,
					managedCluster, managedCluster,
					works, crdv1Work, endpointMetricsOperatorDeploy, hubInfoSecret, true)
			} else {
				err = createManagedClusterRes(c, restMapper, mco,
					managedCluster, managedCluster,
					works, crdv1Work, endpointMetricsOperatorDeploy, hubInfoSecret, false)
			}
			if err != nil {
				failedCreateManagedClusterRes = true
				log.Error(err, "Failed to create managedcluster resources", "namespace", managedCluster)
			}
			if request.Namespace == managedCluster {
				break
			}
		}
	}

	failedDeleteOba := false
	for _, cluster := range currentClusters {
		log.Info("To delete observabilityAddon", "namespace", cluster)
		err = deleteObsAddon(c, cluster)
		if err != nil {
			failedDeleteOba = true
			log.Error(err, "Failed to delete observabilityaddon", "namespace", cluster)
		}
	}

	if failedCreateManagedClusterRes || failedDeleteOba {
		return ctrl.Result{}, errors.New("Failed to create managedcluster resources or" +
			" failed to delete observabilityaddon, skip and reconcile later")
	}

	return ctrl.Result{}, nil
}

func deleteAllObsAddons(
	client client.Client,
	obsAddonList *mcov1beta1.ObservabilityAddonList) (ctrl.Result, error) {
	for _, ep := range obsAddonList.Items {
		err := deleteObsAddon(client, ep.Namespace)
		if err != nil {
			log.Error(err, "Failed to delete observabilityaddon", "namespace", ep.Namespace)
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func deleteGlobalResource(c client.Client) error {
	err := deleteClusterRole(c)
	if err != nil {
		return err
	}
	err = deleteResourceRole(c)
	if err != nil {
		return err
	}
	isCRoleCreated = false
	//delete ClusterManagementAddon
	err = util.DeleteClusterManagementAddon(c)
	if err != nil {
		return err
	}
	isClusterManagementAddonCreated = false
	return nil
}

func createManagedClusterRes(client client.Client, restMapper meta.RESTMapper,
	mco *mcov1beta2.MultiClusterObservability, name string, namespace string,
	works []workv1.Manifest, crdWork *workv1.Manifest, dep *appsv1.Deployment,
	hubInfo *corev1.Secret, installProm bool) error {
	err := createObsAddon(client, namespace)
	if err != nil {
		log.Error(err, "Failed to create observabilityaddon")
		return err
	}

	err = createRolebindings(client, namespace, name)
	if err != nil {
		return err
	}

	err = createManifestWorks(client, restMapper, namespace, name, mco, works, crdWork, dep, hubInfo, installProm)
	if err != nil {
		log.Error(err, "Failed to create manifestwork")
		return err
	}

	err = util.CreateManagedClusterAddonCR(client, namespace, ownerLabelKey, ownerLabelValue)
	if err != nil {
		log.Error(err, "Failed to create ManagedClusterAddon")
		return err
	}

	return nil
}

func deleteManagedClusterRes(c client.Client, namespace string) error {

	managedclusteraddon := &addonv1alpha1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.ManagedClusterAddonName,
			Namespace: namespace,
		},
	}
	err := c.Delete(context.TODO(), managedclusteraddon)
	if err != nil && !k8serrors.IsNotFound(err) {
		log.Error(err, "Failed to delete managedclusteraddon")
		return err
	}

	err = deleteRolebindings(c, namespace)
	if err != nil {
		return err
	}

	err = deleteManifestWorks(c, namespace)
	if err != nil {
		log.Error(err, "Failed to delete manifestwork")
		return err
	}
	return nil
}

func updateManagedClusterList(obj client.Object) {
	if version, ok := obj.GetLabels()["openshiftVersion"]; ok {
		managedClusterList[obj.GetName()] = version
	} else {
		managedClusterList[obj.GetName()] = nonOCP
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ObservabilityAgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c := mgr.GetClient()
	//ingressCtlCrdExists, _ := r.CRDMap[operatorsconfig.IngressControllerCRD]
	clusterPred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			log.Info("CreateFunc", "managedCluster", e.Object.GetName())
			updateManagedClusterList(e.Object)
			updateManagedClusterImageRegistry(e.Object)
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			log.Info("UpdateFunc", "managedCluster", e.ObjectNew.GetName())
			if e.ObjectNew.GetResourceVersion() != e.ObjectOld.GetResourceVersion() {
				if e.ObjectNew.GetDeletionTimestamp() != nil {
					log.Info("managedcluster is in terminating state", "managedCluster", e.ObjectNew.GetName())
					delete(managedClusterList, e.ObjectNew.GetName())
					delete(managedClusterImageRegistry, e.ObjectNew.GetName())
				} else {
					updateManagedClusterList(e.ObjectNew)
					updateManagedClusterImageRegistry(e.ObjectNew)
				}
				return true
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			log.Info("DeleteFunc", "managedCluster", e.Object.GetName())
			delete(managedClusterList, e.Object.GetName())
			delete(managedClusterImageRegistry, e.Object.GetName())
			return true
		},
	}

	obsAddonPred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectNew.GetName() == obsAddonName &&
				e.ObjectNew.GetLabels()[ownerLabelKey] == ownerLabelValue &&
				!reflect.DeepEqual(e.ObjectNew.(*mcov1beta1.ObservabilityAddon).Status.Conditions,
					e.ObjectOld.(*mcov1beta1.ObservabilityAddon).Status.Conditions) {
				return true
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if e.Object.GetName() == obsAddonName &&
				e.Object.GetLabels()[ownerLabelKey] == ownerLabelValue {
				log.Info("DeleteFunc", "obsAddonNamespace", e.Object.GetNamespace(), "obsAddonName", e.Object.GetName())
				/* #nosec */
				removePostponeDeleteAnnotationForManifestwork(c, e.Object.GetNamespace())
				return true
			}
			return false
		},
	}

	ctrBuilder := ctrl.NewControllerManagedBy(mgr).
		// Watch for changes to primary resource ManagedCluster with predicate
		For(&clusterv1.ManagedCluster{}, builder.WithPredicates(clusterPred)).
		// secondary watch for observabilityaddon
		Watches(&source.Kind{Type: &mcov1beta1.ObservabilityAddon{}}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(obsAddonPred))
		// secondary watch for MCO
		// Watches(&source.Kind{Type: &mcov1beta2.MultiClusterObservability{}}, handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		// 	return []reconcile.Request{
		// 		{NamespacedName: types.NamespacedName{
		// 			Name: operatorsconfig.MCOUpdatedRequestName,
		// 		}},
		// 	}
		// }), builder.WithPredicates(mcoPred)).
		// secondary watch for custom allowlist configmap
		//Watches(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(customAllowlistPred)).
		// secondary watch for certificate secrets
		//Watches(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(certSecretPred)).
		// secondary watch for alertmanager accessor serviceaccount
		//Watches(&source.Kind{Type: &corev1.ServiceAccount{}}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(amAccessorSAPred))

	manifestWorkGroupKind := schema.GroupKind{Group: workv1.GroupVersion.Group, Kind: "ManifestWork"}
	if _, err := r.RESTMapper.RESTMapping(manifestWorkGroupKind, workv1.GroupVersion.Version); err == nil {
		workPred := predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				if e.ObjectNew.GetLabels()[ownerLabelKey] == ownerLabelValue &&
					e.ObjectNew.GetResourceVersion() != e.ObjectOld.GetResourceVersion() &&
					!reflect.DeepEqual(e.ObjectNew.(*workv1.ManifestWork).Spec.Workload.Manifests,
						e.ObjectOld.(*workv1.ManifestWork).Spec.Workload.Manifests) {
					return true
				}
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return e.Object.GetLabels()[ownerLabelKey] == ownerLabelValue
			},
		}

		// secondary watch for manifestwork
		ctrBuilder = ctrBuilder.Watches(&source.Kind{Type: &workv1.ManifestWork{}}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(workPred))
	}

	mchGroupKind := schema.GroupKind{Group: mchv1.SchemeGroupVersion.Group, Kind: "MultiClusterHub"}
	if _, err := r.RESTMapper.RESTMapping(mchGroupKind, mchv1.SchemeGroupVersion.Version); err == nil {
		mchPred := predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				// this is for operator restart, the mch CREATE event will be caught and the mch should be ready
				if e.Object.GetNamespace() == operatorsconfig.GetMCONamespace() &&
					e.Object.(*mchv1.MultiClusterHub).Status.CurrentVersion != "" &&
					e.Object.(*mchv1.MultiClusterHub).Status.DesiredVersion == e.Object.(*mchv1.MultiClusterHub).Status.CurrentVersion {
					// only read the image manifests configmap and enqueue the request when the MCH is installed/upgraded successfully
					ok, err := operatorsconfig.ReadImageManifestConfigMap(c, e.Object.(*mchv1.MultiClusterHub).Status.CurrentVersion)
					if err != nil {
						return false
					}
					return ok
				}
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				if e.ObjectNew.GetNamespace() == operatorsconfig.GetMCONamespace() &&
					e.ObjectNew.GetResourceVersion() != e.ObjectOld.GetResourceVersion() &&
					e.ObjectNew.(*mchv1.MultiClusterHub).Status.CurrentVersion != "" &&
					e.ObjectNew.(*mchv1.MultiClusterHub).Status.DesiredVersion == e.ObjectNew.(*mchv1.MultiClusterHub).Status.CurrentVersion {
					/// only read the image manifests configmap and enqueue the request when the MCH is installed/upgraded successfully
					ok, err := operatorsconfig.ReadImageManifestConfigMap(c, e.ObjectNew.(*mchv1.MultiClusterHub).Status.CurrentVersion)
					if err != nil {
						return false
					}
					return ok
				}
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
		}

		// if ingressCtlCrdExists {
		// 	// secondary watch for default ingresscontroller
		// 	ctrBuilder = ctrBuilder.Watches(&source.Kind{Type: &operatorv1.IngressController{}}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(ingressControllerPred)).
		// 		// secondary watch for alertmanager route byo cert secrets
		// 		Watches(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(amRouterCertSecretPred)).
		// 		// secondary watch for openshift route ca secret
		// 		Watches(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(routeCASecretPred))
		// }

		mchCrdExists, _ := r.CRDMap[operatorsconfig.MCHCrdName]
		if mchCrdExists {
			// secondary watch for MCH
			ctrBuilder = ctrBuilder.Watches(&source.Kind{Type: &mchv1.MultiClusterHub{}}, handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
				return []reconcile.Request{
					{NamespacedName: types.NamespacedName{
						Name:      operatorsconfig.MCHUpdatedRequestName,
						Namespace: obj.GetNamespace(),
					}},
				}
			}), builder.WithPredicates(mchPred))
		}
	}

	// create and return a new controller
	return ctrBuilder.Complete(r)
}

/*

	ingressControllerPred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Object.GetName() == operatorsconfig.OpenshiftIngressOperatorCRName &&
				e.Object.GetNamespace() == operatorsconfig.OpenshiftIngressOperatorNamespace {
				// generate the hubInfo secret
				hubInfoSecret, _ = generateHubInfoSecret(c, operatorsconfig.GetDefaultNamespace(), spokeNameSpace, ingressCtlCrdExists)
				return true
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectNew.GetName() == operatorsconfig.OpenshiftIngressOperatorCRName &&
				e.ObjectNew.GetResourceVersion() != e.ObjectOld.GetResourceVersion() &&
				e.ObjectNew.GetNamespace() == operatorsconfig.OpenshiftIngressOperatorNamespace {
				// regenerate the hubInfo secret
				hubInfoSecret, _ = generateHubInfoSecret(c, operatorsconfig.GetDefaultNamespace(), spokeNameSpace, ingressCtlCrdExists)
				return true
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if e.Object.GetName() == operatorsconfig.OpenshiftIngressOperatorCRName &&
				e.Object.GetNamespace() == operatorsconfig.OpenshiftIngressOperatorNamespace {
				// regenerate the hubInfo secret
				hubInfoSecret, _ = generateHubInfoSecret(c, operatorsconfig.GetDefaultNamespace(), spokeNameSpace, ingressCtlCrdExists)
				return true
			}
			return false
		},
	}

	amRouterCertSecretPred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Object.GetNamespace() == operatorsconfig.GetDefaultNamespace() &&
				(e.Object.GetName() == operatorsconfig.AlertmanagerRouteBYOCAName ||
					e.Object.GetName() == operatorsconfig.AlertmanagerRouteBYOCERTName) {
				// generate the hubInfo secret
				hubInfoSecret, _ = generateHubInfoSecret(c, operatorsconfig.GetDefaultNamespace(), spokeNameSpace, ingressCtlCrdExists)
				return true
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectNew.GetNamespace() == operatorsconfig.GetDefaultNamespace() &&
				e.ObjectNew.GetResourceVersion() != e.ObjectOld.GetResourceVersion() &&
				(e.ObjectNew.GetName() == operatorsconfig.AlertmanagerRouteBYOCAName ||
					e.ObjectNew.GetName() == operatorsconfig.AlertmanagerRouteBYOCERTName) {
				// regenerate the hubInfo secret
				hubInfoSecret, _ = generateHubInfoSecret(c, operatorsconfig.GetDefaultNamespace(), spokeNameSpace, ingressCtlCrdExists)
				return true
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if e.Object.GetNamespace() == operatorsconfig.GetDefaultNamespace() &&
				(e.Object.GetName() == operatorsconfig.AlertmanagerRouteBYOCAName ||
					e.Object.GetName() == operatorsconfig.AlertmanagerRouteBYOCERTName) {
				// regenerate the hubInfo secret
				hubInfoSecret, _ = generateHubInfoSecret(c, operatorsconfig.GetDefaultNamespace(), spokeNameSpace, ingressCtlCrdExists)
				return true
			}
			return false
		},
	}

	routeCASecretPred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if (e.Object.GetNamespace() == operatorsconfig.OpenshiftIngressOperatorNamespace &&
				e.Object.GetName() == operatorsconfig.OpenshiftIngressRouteCAName) ||
				(e.Object.GetNamespace() == operatorsconfig.OpenshiftIngressNamespace &&
					e.Object.GetName() == operatorsconfig.OpenshiftIngressDefaultCertName) {
				// generate the hubInfo secret
				hubInfoSecret, _ = generateHubInfoSecret(c, operatorsconfig.GetDefaultNamespace(), spokeNameSpace, ingressCtlCrdExists)
				return true
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if ((e.ObjectNew.GetNamespace() == operatorsconfig.OpenshiftIngressOperatorNamespace &&
				e.ObjectNew.GetName() == operatorsconfig.OpenshiftIngressRouteCAName) ||
				(e.ObjectNew.GetNamespace() == operatorsconfig.OpenshiftIngressNamespace &&
					e.ObjectNew.GetName() == operatorsconfig.OpenshiftIngressDefaultCertName)) &&
				e.ObjectNew.GetResourceVersion() != e.ObjectOld.GetResourceVersion() {
				// regenerate the hubInfo secret
				hubInfoSecret, _ = generateHubInfoSecret(c, operatorsconfig.GetDefaultNamespace(), spokeNameSpace, ingressCtlCrdExists)
				return true
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}

*/
