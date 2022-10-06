/*
 * types.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package k8s

import (
	"chain4travel.com/grungni/pkg/version1"
)

const (
	NODE_ID_KEY = "Node-ID"
)

type stateFullSetOptions struct {
	version1.K8sConfig
	Type        string
	IsValidator bool
	IsRoot      bool
	Replicas    int32
}

func (s stateFullSetOptions) Name() string {
	return s.PrefixWith(s.Type)
}

func (s stateFullSetOptions) Labels() map[string]string {
	labels := s.K8sConfig.Labels
	labels["type"] = s.Type
	return labels
}
