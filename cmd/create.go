/*
 * create.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package cmd

import (
	"fmt"

	"chain4travel.com/kopernikus/pkg"
	"chain4travel.com/kopernikus/pkg/version1"
	"chain4travel.com/kopernikus/pkg/version1/k8s"
	"github.com/spf13/cobra"
)

func init() {

	createCmd.Flags().Uint64("api-nodes", 2, "number of api-nodes")
	createCmd.Flags().Uint64("validators", 5, "number of validators to create (cannot be higher than the initial generated number)")
	createCmd.Flags().String("image", "c4tplatform/camino-node:v0.2.1-rc2", "docker image to run the nodes")

}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "creates the k8s configuration and lauches the network",
	RunE: func(cmd *cobra.Command, args []string) error {

		netorkName, err := cmd.Flags().GetString("network-name")
		if err != nil {
			return err
		}

		kubeconfig, err := cmd.Flags().GetString("kubeconfig")
		if err != nil {
			return err
		}

		image, err := cmd.Flags().GetString("image")
		if err != nil {
			return err
		}

		k8sConfig := version1.K8sConfig{
			K8sPrefix: netorkName,
			Namespace: netorkName,
			Labels: map[string]string{
				"network": netorkName,
			},
			Image:  image,
			Domain: "kopernikus.camino.foundation",
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

		network, err := version1.LoadNetwork(fmt.Sprintf("%s.json", netorkName))
		if err != nil {
			return err
		}
		if int(numValidators) > len(network.Stakers)-1 {
			return fmt.Errorf("network does not contain enought stakers %d > %d", numValidators, len(network.Stakers)-1)
		}

		err = k8s.CreateNamespace(cmd.Context(), k, k8sConfig)
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

		err = k8s.RegisterValidators(cmd.Context(), kRest, k8sConfig, network.Stakers[1:numValidators])
		if err != nil {
			return err
		}

		return nil
	},
}
