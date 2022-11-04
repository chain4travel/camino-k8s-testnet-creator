/*
 * create.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package cmd

import (
	"context"
	"fmt"
	"time"

	"chain4travel.com/camktncr/pkg"
	"chain4travel.com/camktncr/pkg/version1"
	"chain4travel.com/camktncr/pkg/version1/k8s"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func init() {
	createCmd.Flags().Uint64("api-nodes", 2, "number of api-nodes")
	createCmd.Flags().Uint64("validators", 5, "number of validators to create (cannot be higher than the initial generated number)")
	createCmd.Flags().String("validator-ram", "1Gi", "ram of the validators")
	createCmd.Flags().String("validator-cpu", "500m", "cpu of the validators")
	createCmd.Flags().String("api-nodes-ram", "1Gi", "ram of the api-nodes")
	createCmd.Flags().String("api-nodes-cpu", "500m", "cpu of the api-nodes")
	createCmd.Flags().String("tls-secret-name", "kopernikus.camino.foundation-ingress-tls", "tls secret located in default namespace")
	createCmd.Flags().String("pull-secret-name", "gcr-image-pull", "pull secret located in default namespace")
	createCmd.Flags().String("image", "c4tplatform/camino-node:v0.2.1-rc2", "docker image to run the nodes")
	createCmd.Flags().String("domain", "kopernikus.camino.foundation", "under which domain to publish the network api nodes")
	createCmd.Flags().DurationP("timeout", "t", 0, "stop execution after this time (non negative and 0 means no timeout)")
	createCmd.Flags().Bool("enable-monitoring", true, "toggle the creation of service monitors")
}

var createCmd = &cobra.Command{
	Use:   "create <network-name>",
	Short: "creates the k8s configuration and lauches the network",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		networkName := args[0]

		kubeconfig, err := cmd.Flags().GetString("kubeconfig")
		if err != nil {
			return err
		}

		image, err := cmd.Flags().GetString("image")
		if err != nil {
			return err
		}

		domain, err := cmd.Flags().GetString("domain")
		if err != nil {
			return err
		}

		validatorCpu, err := cmd.Flags().GetString("validator-cpu")
		if err != nil {
			return err
		}
		validatorRam, err := cmd.Flags().GetString("validator-ram")
		if err != nil {
			return err
		}
		apiCpu, err := cmd.Flags().GetString("api-nodes-cpu")
		if err != nil {
			return err
		}
		apiRam, err := cmd.Flags().GetString("api-nodes-ram")
		if err != nil {
			return err
		}

		tlsSecretName, err := cmd.Flags().GetString("tls-secret-name")
		if err != nil {
			return err
		}

		pullSecretName, err := cmd.Flags().GetString("pull-secret-name")
		if err != nil {
			return err
		}

		enableMonitoring, err := cmd.Flags().GetBool("enable-monitoring")
		if err != nil {
			return err
		}

		k8sConfig := version1.K8sConfig{
			K8sPrefix: networkName,
			Namespace: networkName,
			Labels: map[string]string{
				"network": networkName,
			},
			Image:          image,
			Domain:         domain,
			TLSSecretName:  tlsSecretName,
			PullSecretName: pullSecretName,
			Resources: version1.K8sResources{
				Api: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse(apiCpu),
					v1.ResourceMemory: resource.MustParse(apiRam),
				},
				Validator: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse(validatorCpu),
					v1.ResourceMemory: resource.MustParse(validatorRam),
				},
			},
			EnableMonitoring: enableMonitoring,
		}

		numValidators, err := cmd.Flags().GetUint64("validators")
		if err != nil {
			return err
		}

		numApiNodes, err := cmd.Flags().GetUint64("api-nodes")
		if err != nil {
			return err
		}

		timeoutDur, err := cmd.Flags().GetDuration("timeout")
		if err != nil {
			return err
		}
		ctx := cmd.Context()
		if timeoutDur > 0 {
			ctx, _ = context.WithTimeout(ctx, timeoutDur)
		}

		kRest, k, err := pkg.InitClientSet(kubeconfig)
		if err != nil {
			return err
		}

		network, err := version1.LoadNetwork(fmt.Sprintf("%s.json", networkName))
		if err != nil {
			return err
		}

		if network.Version == "" {
			return fmt.Errorf("using old network json, please regenerate")
		}

		if network.Version != pkg.Commit {
			return fmt.Errorf("cannot create network with different version, please checkout this commit and use that version to create the network: %s", network.Version)
		}

		numInitialStakers := len(network.GenesisConfig.InitialStakers)

		if int(numValidators) < numInitialStakers {
			return fmt.Errorf("network needs at least all initial stakers to be started: %d < %d", numValidators, numInitialStakers)
		}

		if int(numValidators) > len(network.Stakers) {
			return fmt.Errorf("network config '%s' does not contain enough validators: %d > %d", networkName, numValidators, len(network.Stakers))
		}

		err = k8s.CreateNamespace(cmd.Context(), k, k8sConfig)
		if err != nil {
			return err
		}

		err = k8s.CopySecretFromDefaultNamespace(ctx, k, k8sConfig, pullSecretName)
		if err != nil {
			return err
		}
		err = k8s.CopySecretFromDefaultNamespace(ctx, k, k8sConfig, tlsSecretName)
		if err != nil {
			return err
		}
		err = k8s.CreateRBAC(ctx, k, k8sConfig)
		if err != nil {
			return err
		}

		now := time.Now().Unix()
		genesisConfig := version1.BuildGenesisConfig(network.GenesisConfig.Allocations, uint64(now), network.Stakers[:numValidators], networkName)

		// err = k8s.CreateNetworkConfigMap(ctx, k, network.GenesisConfig, k8sConfig)
		err = k8s.CreateNetworkConfigMap(ctx, k, genesisConfig, k8sConfig)
		if err != nil {
			return err
		}

		err = k8s.CreateScriptsConfigMap(ctx, k, k8sConfig)
		if err != nil {
			return err
		}

		err = k8s.CreateStakerSecrets(ctx, k, network.Stakers, k8sConfig)
		if err != nil {
			return err
		}

		err = k8s.CreateRootNode(ctx, kRest, k, k8sConfig)
		if err != nil {
			return err
		}

		err = k8s.CreateValidators(ctx, kRest, k, k8sConfig, int32(numValidators)-1)
		if err != nil {
			return err
		}

		err = k8s.CreateApiNodes(ctx, kRest, k, k8sConfig, int32(numApiNodes))
		if err != nil {
			return err
		}

		ingAnnotations := map[string]string{
			// "cert-manager.io/cluster-issuer": "prod-letsencrypt",
		}

		err = k8s.CreateIngress(ctx, k, k8sConfig, ingAnnotations)
		if err != nil {
			return err
		}

		err = k8s.RegisterValidators(ctx, kRest, k8sConfig, network.Stakers[numInitialStakers:numValidators], true)
		if err != nil {
			return err
		}

		return nil
	},
}
