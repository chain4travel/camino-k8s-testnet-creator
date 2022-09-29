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

type K8sConfig struct {
	K8sPrefix string
	Namespace string
	Domain    string
	Labels    map[string]string
	Image     string
}

func (k K8sConfig) PrefixWith(s string) string {
	return fmt.Sprintf("%s-%s", k.K8sPrefix, s)
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
	GenesisConfig genesis.UnparsedConfig
	Stakers       []Staker
}
