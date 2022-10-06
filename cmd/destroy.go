/*
 * destroy.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package cmd

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"chain4travel.com/grungni/pkg"

	"github.com/spf13/cobra"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "destroy the cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		netorkName, err := cmd.Flags().GetString("network-name")
		if err != nil {
			return err
		}

		kubeconfig, err := cmd.Flags().GetString("kubeconfig")
		if err != nil {
			return err
		}

		_, k, err := pkg.InitClientSet(kubeconfig)
		if err != nil {
			return err
		}

		err = k.CoreV1().Namespaces().Delete(cmd.Context(), netorkName, *metav1.NewDeleteOptions(0))
		if err != nil && !k8sErrors.IsNotFound(err) {
			return err
		}

		_, err = k.CoreV1().Namespaces().Get(cmd.Context(), netorkName, metav1.GetOptions{})
		if err == nil {
			w, err := k.CoreV1().Namespaces().Watch(cmd.Context(), metav1.SingleObject(metav1.ObjectMeta{Name: netorkName}))
			if err != nil {
				return err
			}

			for event := range w.ResultChan() {
				if event.Type == watch.Deleted {
					w.Stop()
				}

				fmt.Printf("waiting for %s to be deleted\n", netorkName)

			}
		} else if !k8sErrors.IsNotFound(err) {
			return err
		}

		return nil
	},
}