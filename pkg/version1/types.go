/*
 * types.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package version1

import (
	"crypto/tls"
	"fmt"

	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/ids"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Staker struct {
	NodeID        ids.NodeID
	Cert          tls.Certificate `json:"-"`
	CertBytes     []byte
	KeyBytes      []byte
	Stake         uint64
	PrivateKey    string
	PublicAddress string
}

type NetworkConfig struct {
	NumStakers        uint64
	NumInitialStakers uint64
	NetworkName       string
	NetworkID         uint64
	DefaultStake      uint64
}

type K8sResources struct {
	Api       corev1.ResourceList
	Validator corev1.ResourceList
}

type K8sConfig struct {
	K8sPrefix        string
	Namespace        string
	Domain           string
	Labels           map[string]string
	Image            string
	TLSSecretName    string
	PullSecretName   string
	Resources        K8sResources
	EnableMonitoring bool
}

func (k K8sConfig) PrefixWith(s string) string {
	return fmt.Sprintf("%s-%s", k.K8sPrefix, s)
}

func (k K8sConfig) Selector() *metav1.LabelSelector {

	sel := &metav1.LabelSelector{}

	for k, v := range k.Labels {
		sel = metav1.AddLabelToSelector(sel, k, v)
	}

	// sel := labels.NewSelector()
	// for k, v := range k.Labels {
	// 	req, err := labels.NewRequirement(k, selection.Equals, []string{v})
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	sel.Add(*req)
	// }
	return sel
}

type stakerTemplate struct {
	Staker      Staker
	StakeTime   uint64
	Username    string
	Password    string
	StakeAmount uint64
	Address     string
}

type Network struct {
	Version       string
	GenesisConfig genesis.UnparsedConfig
	Stakers       []Staker
}
