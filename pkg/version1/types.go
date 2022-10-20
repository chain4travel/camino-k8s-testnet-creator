/*
 * types.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package version1

import (
	"crypto/tls"
	"fmt"

	"github.com/chain4travel/caminogo/genesis"
	"github.com/chain4travel/caminogo/ids"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type Staker struct {
	NodeID        ids.ShortID
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
	K8sPrefix      string
	Namespace      string
	Domain         string
	Labels         map[string]string
	Image          string
	TLSSecretName  string
	PullSecretName string
	Resources      K8sResources
}

func (k K8sConfig) PrefixWith(s string) string {
	return fmt.Sprintf("%s-%s", k.K8sPrefix, s)
}

func (k K8sConfig) Selector() (string, error) {
	sel := labels.NewSelector()
	for k, v := range k.Labels {
		req, err := labels.NewRequirement(k, selection.Equals, []string{v})
		if err != nil {
			return "", err
		}
		sel.Add(*req)
	}
	return sel.String(), nil
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
