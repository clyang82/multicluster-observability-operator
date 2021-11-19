// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package certificates

import (
	"errors"
	"time"

	"github.com/cloudflare/cfssl/config"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	certificatesv1 "k8s.io/api/certificates/v1"

	agentconfig "github.com/open-cluster-management/multicluster-observability-operator/operators/observabilityagent/pkg/config"
	"github.com/open-cluster-management/multicluster-observability-operator/operators/pkg/util"
)

var (
	clientCACertificateCN = "observability-client-ca-certificate"
	clientCACerts         = "observability-client-ca-certs"
)

func sign(csr *certificatesv1.CertificateSigningRequest) []byte {
	c, err := util.CreateKubeClient(agentconfig.OBSCoreKubeconfigPath, nil)
	if err != nil {
		log.Error(err, "Failed to create the Kube client")
		return nil
	}

	caCert, caKey, _, err := util.GetCA(c, false)
	if err != nil {
		return nil
	}

	var usages []string
	for _, usage := range csr.Spec.Usages {
		usages = append(usages, string(usage))
	}

	certExpiryDuration := 365 * 24 * time.Hour
	durationUntilExpiry := time.Until(caCert.NotAfter)
	if durationUntilExpiry <= 0 {
		log.Error(errors.New("signer has expired"), "the signer has expired", "expired time", caCert.NotAfter)
		return nil
	}
	if durationUntilExpiry < certExpiryDuration {
		certExpiryDuration = durationUntilExpiry
	}

	policy := &config.Signing{
		Default: &config.SigningProfile{
			Usage:        usages,
			Expiry:       certExpiryDuration,
			ExpiryString: certExpiryDuration.String(),
		},
	}
	cfs, err := local.NewSigner(caKey, caCert, signer.DefaultSigAlgo(caKey), policy)
	if err != nil {
		log.Error(err, "Failed to create new local signer")
		return nil
	}

	signedCert, err := cfs.Sign(signer.SignRequest{
		Request: string(csr.Spec.Request),
	})
	if err != nil {
		log.Error(err, "Failed to sign the CSR")
		return nil
	}
	return signedCert
}
