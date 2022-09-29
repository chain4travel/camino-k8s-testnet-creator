/*
 * generate.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"chain4travel.com/kopernikus/pkg/version1"
	"github.com/spf13/cobra"
)

func init() {

	generateCmd.Flags().Uint64("num-stakers", 20, "number of stakers total")
	generateCmd.Flags().Uint64("num-initial-stakers", 1, "number of initial stakers")
	generateCmd.Flags().Uint64("network-id", 12345, "network id")
	generateCmd.Flags().Uint64("default-stake", 2e5, "initial stake for each validator")
	generateCmd.Flags().Bool("override", false, "overwrite and delete existing data")

}

const DENOMINATION = 1e9

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "generates a network with the specified config",
	Long:  `The generation `,
	RunE: func(cmd *cobra.Command, args []string) error {
		netorkName, err := cmd.Flags().GetString("network-name")
		if err != nil {
			return err
		}

		override, err := cmd.Flags().GetBool("override")
		if err != nil {
			return err
		}

		networkPath := fmt.Sprintf("%s.json", netorkName)
		_, err = os.Stat(networkPath)
		if err == nil && !override {
			return fmt.Errorf("will not override existing data without --overide flag")
		}

		defaultStake, err := cmd.Flags().GetUint64("default-stake")
		if err != nil {
			return err
		}
		numStakers, err := cmd.Flags().GetUint64("num-stakers")
		if err != nil {
			return err
		}
		netowrkId, err := cmd.Flags().GetUint64("network-id")
		if err != nil {
			return err
		}
		numInitialStakers, err := cmd.Flags().GetUint64("num-initial-stakers")
		if err != nil {
			return err
		}

		networkConfig := version1.NetworkConfig{
			NumStakers:        numStakers,
			NetworkID:         netowrkId,
			NetworkName:       "custom",
			DefaultStake:      defaultStake * DENOMINATION,
			NumInitialStakers: numInitialStakers,
		}

		now := uint64(time.Now().Unix())
		network := version1.BuildNetwork(networkConfig, now)

		networkJson, err := json.MarshalIndent(network, "", "\t")
		if err != nil {
			return err
		}

		err = os.WriteFile(networkPath, networkJson, 0700)
		if err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}

		return nil
	},
}
