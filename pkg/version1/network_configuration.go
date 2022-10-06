/*
 * network_configuration.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package version1

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	_ "embed"

	"github.com/chain4travel/caminogo/genesis"
	"github.com/chain4travel/caminogo/network/peer"
	"github.com/chain4travel/caminogo/staking"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/utils/formatting"
	"github.com/schollz/progressbar/v3"
)

func createAllocations(stakers []Staker, config NetworkConfig) []genesis.UnparsedAllocation {

	allocations := make([]genesis.UnparsedAllocation, len(stakers))
	for i := 0; i < len(stakers); i++ {
		allocations[i] = genesis.UnparsedAllocation{
			ETHAddr:       "0x0000000000000000000000000000000000000000",
			AVAXAddr:      stakers[i].PublicAddress,
			InitialAmount: 2 * config.DefaultStake,
			UnlockSchedule: []genesis.LockedAmount{
				{
					Amount:   2 * config.DefaultStake,
					Locktime: 2524604400,
				},
			},
		}
	}

	return allocations
}

func createStakers(config NetworkConfig) []Staker {
	stakers := make([]Staker, config.NumStakers)

	bar := progressbar.Default(int64(config.NumStakers))

	factory := crypto.FactorySECP256K1R{}
	for i := 0; i < int(config.NumStakers); i++ {

		CertBytes, KeyBytes, err := staking.NewCertAndKeyBytes()
		if err != nil {
			log.Fatal(err)
		}

		cert, err := staking.LoadTLSCertFromBytes(KeyBytes, CertBytes)
		if err != nil {
			log.Fatal(err)
		}

		nodeID := peer.CertToID(cert.Leaf)

		pk, err := factory.NewPrivateKey()
		if err != nil {
			log.Fatal(err)
		}

		pk_bytes := pk.Bytes()
		pk_string, err := formatting.EncodeWithChecksum(formatting.CB58, pk_bytes[:])
		if err != nil {
			log.Fatal(err)
		}

		pk_with_prefix := fmt.Sprintf("PrivateKey-%s", pk_string)
		addr_bytes := pk.PublicKey().Address()
		addr, err := formatting.FormatAddress("X", config.NetworkName, addr_bytes[:])
		if err != nil {
			log.Fatal(err)
		}

		stakers[i] = Staker{
			nodeID, *cert, CertBytes, KeyBytes, config.DefaultStake, pk_with_prefix, addr,
		}
		bar.Add(1)
	}

	return stakers
}

func BuildNetwork(config NetworkConfig, now uint64) Network {

	stakersRaw := createStakers(config)

	initialStakersUnparsed := stakersRaw[:config.NumInitialStakers]

	allocations := createAllocations(stakersRaw, config)

	initialStakedFunds := make([]string, len(initialStakersUnparsed))
	for i := 0; i < len(initialStakersUnparsed); i++ {
		initialStakedFunds[i] = initialStakersUnparsed[i].PublicAddress
	}

	initialStakers := make([]genesis.UnparsedStaker, len(initialStakersUnparsed))

	for i := 0; i < len(initialStakersUnparsed); i++ {
		initialStakers[i] = genesis.UnparsedStaker{
			NodeID:        fmt.Sprintf("NodeID-%s", initialStakersUnparsed[i].NodeID.String()),
			RewardAddress: initialStakersUnparsed[i].PublicAddress,
		}
	}

	genesisConfig := genesis.UnparsedConfig{
		NetworkID:                  1002,
		Allocations:                allocations,
		StartTime:                  now,
		InitialStakeDuration:       31536000,
		InitialStakeDurationOffset: 5400,
		InitialStakedFunds:         initialStakedFunds,
		InitialStakers:             initialStakers,
		CChainGenesis:              "{\"config\":{\"chainId\":502,\"homesteadBlock\":0,\"daoForkBlock\":0,\"daoForkSupport\":true,\"eip150Block\":0,\"eip150Hash\":\"0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0\",\"eip155Block\":0,\"eip158Block\":0,\"byzantiumBlock\":0,\"constantinopleBlock\":0,\"petersburgBlock\":0,\"istanbulBlock\":0,\"muirGlacierBlock\":0,\"apricotPhase1BlockTimestamp\":0,\"apricotPhase2BlockTimestamp\":0,\"apricotPhase3BlockTimestamp\":0,\"apricotPhase4BlockTimestamp\":0,\"apricotPhase5BlockTimestamp\":0},\"nonce\":\"0x0\",\"timestamp\":\"0x0\",\"extraData\":\"0x00\",\"gasLimit\":\"0x5f5e100\",\"difficulty\":\"0x0\",\"mixHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"coinbase\":\"0x0000000000000000000000000000000000000000\",\"alloc\":{\"0100000000000000000000000000000000000000\":{\"code\":\"0x7300000000000000000000000000000000000000003014608060405260043610603d5760003560e01c80631e010439146042578063b6510bb314606e575b600080fd5b605c60048036036020811015605657600080fd5b503560b1565b60408051918252519081900360200190f35b818015607957600080fd5b5060af60048036036080811015608e57600080fd5b506001600160a01b03813516906020810135906040810135906060013560b6565b005b30cd90565b836001600160a01b031681836108fc8690811502906040516000604051808303818888878c8acf9550505050505015801560f4573d6000803e3d6000fd5b505050505056fea26469706673582212201eebce970fe3f5cb96bf8ac6ba5f5c133fc2908ae3dcd51082cfee8f583429d064736f6c634300060a0033\",\"balance\":\"0x0\"}},\"number\":\"0x0\",\"gasUsed\":\"0x0\",\"parentHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\"}",
		ValidatorBondAmount:        1e12,
		Message:                    config.NetworkName,
	}

	return Network{
		genesisConfig, stakersRaw,
	}
}

func LoadNetwork(path string) (*Network, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out Network
	err = json.Unmarshal(data, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}