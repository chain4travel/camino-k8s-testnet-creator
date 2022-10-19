/*
 * create.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package cmd

import (
	"fmt"

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
	createCmd.Flags().String("validator-cpu", "1000m", "cpu of the validators")
	createCmd.Flags().String("api-nodes-ram", "1Gi", "ram of the api-nodes")
	createCmd.Flags().String("api-nodes-cpu", "1000m", "cpu of the api-nodes")
	createCmd.Flags().String("image", "c4tplatform/camino-node:v0.2.1-rc2", "docker image to run the nodes")
	createCmd.Flags().String("domain", "kopernikus.camino.foundation", "under which domain to publish the network api nodes")

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
		k8sConfig := version1.K8sConfig{
			K8sPrefix: networkName,
			Namespace: networkName,
			Labels: map[string]string{
				"network": networkName,
			},
			Image:  image,
			Domain: domain,
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
		}

		numValidators, err := cmd.Flags().GetUint64("validators")
		if err != nil {
			return err
		}

		numApiNodes, err := cmd.Flags().GetUint64("api-nodes")
		if err != nil {
			return err
		}

		kRest, k, err := pkg.InitClientSet(kubeconfig)
		if err != nil {
			return err
		}

		network, err := version1.LoadNetwork(fmt.Sprintf("%s.json", networkName))
		if err != nil {
			return err
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

		err = k8s.CopyPullSecret(cmd.Context(), k, k8sConfig)
		if err != nil {
			return err
		}
		err = k8s.CreateRBAC(cmd.Context(), k, k8sConfig)
		if err != nil {
			return err
		}

		err = k8s.CreateNetworkConfigMap(cmd.Context(), k, network.GenesisConfig, k8sConfig)
		if err != nil {
			return err
		}

		err = k8s.CreateScriptsConfigMap(cmd.Context(), k, k8sConfig)
		if err != nil {
			return err
		}

		err = k8s.CreateStakerSecrets(cmd.Context(), k, network.Stakers, k8sConfig)
		if err != nil {
			return err
		}

		err = k8s.CreateRootNode(cmd.Context(), k, k8sConfig)
		if err != nil {
			return err
		}

		err = k8s.CreateValidators(cmd.Context(), k, k8sConfig, int32(numValidators)-1)
		if err != nil {
			return err
		}

		err = k8s.CreateApiNodes(cmd.Context(), k, k8sConfig, int32(numApiNodes))
		if err != nil {
			return err
		}

		ingAnnotations := map[string]string{
			"cert-manager.io/cluster-issuer": "prod-letsencrypt",
		}

		err = k8s.CreateIngress(cmd.Context(), k, k8sConfig, ingAnnotations)
		if err != nil {
			return err
		}

		err = k8s.RegisterValidators(cmd.Context(), kRest, k8sConfig, network.Stakers[numInitialStakers:numValidators])
		if err != nil {
			return err
		}

		return nil
	},
}
