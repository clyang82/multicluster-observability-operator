// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package certificates

import (
	"context"
	"os"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"open-cluster-management.io/addon-framework/pkg/addonmanager"
)

var (
	log = logf.Log.WithName("observabilityagent_certificates")
)

func Start(c client.Client) {

	// setup ocm addon manager
	addonMgr, err := addonmanager.New(ctrl.GetConfigOrDie())
	if err != nil {
		log.Error(err, "Failed to init addon manager")
		os.Exit(1)
	}
	agent := &ObservabilityAgent{}
	err = addonMgr.AddAgent(agent)
	if err != nil {
		log.Error(err, "Failed to add agent for addon manager")
		os.Exit(1)
	}

	err = addonMgr.Start(context.TODO())
	if err != nil {
		log.Error(err, "Failed to start addon manager")
		os.Exit(1)
	}
}
