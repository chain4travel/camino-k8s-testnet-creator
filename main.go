/*
 * main.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package main

import (
	"chain4travel.com/kopernikus/cmd"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	cmd.Run()
}
