/*
 * k8s_utils.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package pkg

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func InitClientSet(kubeconfig string) (*rest.Config, *kubernetes.Clientset, error) {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, nil, err
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return config, clientset, nil

}
