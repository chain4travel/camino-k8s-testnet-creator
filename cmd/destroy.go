/*
 * destroy.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package cmd

import (
	"time"

	"chain4travel.com/camktncr/pkg"
	"chain4travel.com/camktncr/pkg/version1"
	"chain4travel.com/camktncr/pkg/version1/k8s"

	"github.com/spf13/cobra"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy <network-name>",
	Short: "destroy the cluster",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		networkName := args[0]

		kubeconfig, err := cmd.Flags().GetString("kubeconfig")
		if err != nil {
			return err
		}

		_, k, err := pkg.InitClientSet(kubeconfig)
		if err != nil {
			return err
		}

		k8sConfig := version1.K8sConfig{
			K8sPrefix: networkName,
			Namespace: networkName,
			Labels: map[string]string{
				"network": networkName,
			},
		}

		err = k8s.DeleteCluster(cmd.Context(), k, k8sConfig, false)
		if err != nil {
			return err
		}

		time.Sleep(20 * time.Second)

		return nil
	},
}
