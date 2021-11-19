// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/IBM/controller-filtered-cache/filteredcache"
	operatorv1 "github.com/openshift/api/operator/v1"
	oinformers "github.com/openshift/client-go/operator/informers/externalversions"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	observabilityagentctl "github.com/open-cluster-management/multicluster-observability-operator/operators/observabilityagent/controllers/observabilityagent"
	agentconfig "github.com/open-cluster-management/multicluster-observability-operator/operators/observabilityagent/pkg/config"
	mcoinformers "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/apis/multiclusterobservability/client/informers/externalversions"
	observabilityv1beta1 "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/apis/multiclusterobservability/v1beta1"
	observabilityv1beta2 "github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/apis/multiclusterobservability/v1beta2"
	"github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/config"
	"github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/util"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	// +kubebuilder:scaffold:imports
)

var (
	metricsHost       = "0.0.0.0"
	metricsPort int32 = 8383

	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(observabilityv1beta1.AddToScheme(scheme))
	utilruntime.Must(observabilityv1beta2.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		// enable development mode for more human-readable output, extra stack traces and logging information, etc
		// disable this in final release
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	crdClient, err := util.GetOrCreateCRDClient()
	if err != nil {
		setupLog.Error(err, "Failed to create the CRD client")
		os.Exit(1)
	}

	if err := operatorv1.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}

	if err := workv1.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}

	if err := clusterv1.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}

	if err := addonv1alpha1.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}

	mcoCrdExists, err := util.CheckCRDExist(crdClient, config.MCOCrdName)
	if err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}
	if mcoCrdExists {
		if err := observabilityv1beta2.SchemeBuilder.AddToScheme(scheme); err != nil {
			setupLog.Error(err, "")
			os.Exit(1)
		}
	}

	gvkLabelsMap := map[schema.GroupVersionKind][]filteredcache.Selector{
		workv1.SchemeGroupVersion.WithKind("ManifestWork"): []filteredcache.Selector{
			{LabelSelector: "owner==multicluster-observability-operator"},
		},
		clusterv1.SchemeGroupVersion.WithKind("ManagedCluster"): []filteredcache.Selector{
			{LabelSelector: "vendor!=auto-detect,observability!=disabled"},
		},
	}

	// Add filter for ManagedClusterAddOn to reduce the cache size when the managedclusters scale.
	gvkLabelsMap[addonv1alpha1.SchemeGroupVersion.WithKind("ManagedClusterAddOn")] = []filteredcache.Selector{
		{LabelSelector: "owner==multicluster-observability-operator"},
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     fmt.Sprintf("%s:%d", metricsHost, metricsPort),
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "b9d51391.open-cluster-management.io",
		NewCache:               filteredcache.NewEnhancedFilteredCacheBuilder(gvkLabelsMap),
	})

	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	crdMaps := map[string]bool{
		config.MCOCrdName: mcoCrdExists,
	}

	kubeClient, err := util.CreateKubeClient(agentconfig.OBSCoreKubeconfigPath, scheme)
	if err != nil {
		setupLog.Error(err, "Failed to create the Kube client")
		os.Exit(1)
	}

	ocpRouteClientset, err := util.CreateOCPRouteClient(agentconfig.OBSCoreKubeconfigPath)
	if err != nil {
		setupLog.Error(err, "Failed to create the OCP clientset")
		os.Exit(1)
	}

	ocpOperatorClientset, err := util.CreateOCPOperatorClientset(agentconfig.OBSCoreKubeconfigPath)
	if err != nil {
		setupLog.Error(err, "Failed to create the ocp operator clientset")
		os.Exit(1)
	}

	if err = (&observabilityagentctl.ObservabilityAgentReconciler{
		Client:            mgr.GetClient(),
		KubeClient:        kubeClient,
		RouteClientset:    *ocpRouteClientset,
		OperatorClientset: *ocpOperatorClientset,
		Log:               ctrl.Log.WithName("controllers").WithName("ObservabilityAgent"),
		Scheme:            mgr.GetScheme(),
		CRDMap:            crdMaps,
		RESTMapper:        mgr.GetRESTMapper(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ObservabilityAgent")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	if err := util.RegisterDebugEndpoint(mgr.AddMetricsExtraHandler); err != nil {
		setupLog.Error(err, "unable to set up debug handler")
		os.Exit(1)
	}

	kubeClientset, err := util.CreateKubeClientset(agentconfig.OBSCoreKubeconfigPath)
	if err != nil {
		setupLog.Error(err, "Failed to create the Kube client")
		os.Exit(1)
	}

	mcoClientset, err := util.CreateMCOClientset(agentconfig.OBSCoreKubeconfigPath)
	if err != nil {
		setupLog.Error(err, "Failed to create the ocp client")
		os.Exit(1)
	}

	setupLog.Info("add watch observability-core-controller to manager")
	controller := observabilityagentctl.NewObservabilityCoreController(
		informers.NewSharedInformerFactory(kubeClientset, 0),
		oinformers.NewSharedInformerFactory(ocpOperatorClientset, 0),
		mcoinformers.NewSharedInformerFactory(mcoClientset, 0),
		kubeClient, *ocpRouteClientset, *ocpOperatorClientset)
	if err := mgr.Add(controller); err != nil {
		setupLog.Error(err, "unable to add webhook controller to manager")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
