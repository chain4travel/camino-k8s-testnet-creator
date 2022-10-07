/*
 * root.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package cmd

import (
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/client-go/util/homedir"
)

var rootCmd = &cobra.Command{Use: "camktncr", SilenceUsage: true}
var k8sCmd = &cobra.Command{Use: "k8s"}

func init() {

	k8sCmd.AddCommand(createCmd, destroyCmd)

	if home := homedir.HomeDir(); home != "" {
		k8sCmd.PersistentFlags().String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		k8sCmd.PersistentFlags().String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	rootCmd.AddCommand(k8sCmd)
	rootCmd.AddCommand(generateCmd)

}

func Run() {
	rootCmd.Execute()
}
