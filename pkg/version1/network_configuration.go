/*
 * network_configuration.go
 * Copyright (C) 2022, Chain4Travel AG. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package version1

import (
	"encoding/json"
	"fmt"
	"os"

	_ "embed"

	"chain4travel.com/camktncr/pkg"
	"github.com/chain4travel/caminogo/genesis"
	"github.com/chain4travel/caminogo/network/peer"
	"github.com/chain4travel/caminogo/staking"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/utils/formatting"
	"github.com/schollz/progressbar/v3"
)

const BOND_AMOUNT = 1e15

func createAllocations(stakers []Staker, config NetworkConfig) []genesis.UnparsedAllocation {

	allocations := make([]genesis.UnparsedAllocation, 0)
	for i := 0; i < len(stakers); i++ {
		allocations = append(allocations, genesis.UnparsedAllocation{
			ETHAddr:       "0x0000000000000000000000000000000000000000",
			AVAXAddr:      stakers[i].PublicAddress,
			InitialAmount: BOND_AMOUNT,
			UnlockSchedule: []genesis.LockedAmount{
				{
					Amount:   BOND_AMOUNT,
					Locktime: 2524604400,
				},
			},
		})
		allocations = append(allocations, genesis.UnparsedAllocation{
			ETHAddr:        "0x0000000000000000000000000000000000000000",
			AVAXAddr:       stakers[i].PublicAddress,
			InitialAmount:  config.DefaultStake,
			UnlockSchedule: []genesis.LockedAmount{},
		})
	}

	// for i := 0; i < 6000; i++ {v
	// 	var rand_bytes [20]byte
	// 	_, err := rand.Read(rand_bytes[:])
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	addr, err := formatting.FormatAddress("X", "custom", rand_bytes[:])
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	allocations = append(allocations, genesis.UnparsedAllocation{
	// 		ETHAddr:        "0x0000000000000000000000000000000000000000",
	// 		AVAXAddr:       addr,
	// 		InitialAmount:  2 * config.DefaultStake,
	// 		UnlockSchedule: []genesis.LockedAmount{},
	// 	})
	// }

	return allocations
}

func createStakers(config NetworkConfig) ([]Staker, error) {
	stakers := make([]Staker, config.NumStakers)

	bar := progressbar.Default(int64(config.NumStakers))

	factory := crypto.FactorySECP256K1R{}
	for i := 0; i < int(config.NumStakers); i++ {

		CertBytes, KeyBytes, err := staking.NewCertAndKeyBytes()
		if err != nil {
			return nil, err
		}

		cert, err := staking.LoadTLSCertFromBytes(KeyBytes, CertBytes)
		if err != nil {
			return nil, err
		}

		nodeID := peer.CertToID(cert.Leaf)
		// if err != nil {
		// 	return nil, err
		// }

		// rsaKey, ok := cert.PrivateKey.(*rsa.PrivateKey)
		// if !ok {
		// 	log.Fatal(fmt.Errorf("failed to cast private key"))
		// }

		// secpKey := nodeid.RsaPrivateKeyToSecp256PrivateKey(rsaKey)
		// pk, err := factory.ToPrivateKey(secpKey.Serialize())
		// if err != nil {
		// 	log.Fatal(err)
		// }

		pk, err := factory.NewPrivateKey()
		if err != nil {
			return nil, err
		}

		pk_bytes := pk.Bytes()
		pk_string, err := formatting.EncodeWithChecksum(formatting.CB58, pk_bytes[:])
		if err != nil {
			return nil, err
		}

		pk_with_prefix := fmt.Sprintf("PrivateKey-%s", pk_string)
		addr_bytes := pk.PublicKey().Address()
		addr, err := formatting.FormatAddress("X", config.NetworkName, addr_bytes[:])
		if err != nil {
			return nil, err
		}

		stakers[i] = Staker{
			nodeID.PrefixedString("NodeID-"), *cert, CertBytes, KeyBytes, BOND_AMOUNT, pk_with_prefix, addr,
		}
		bar.Add(1)
	}

	return stakers, nil
}

func BuildNetwork(config NetworkConfig, now uint64) (*Network, error) {

	stakersRaw, err := createStakers(config)
	if err != nil {
		return nil, err
	}

	allocations := createAllocations(stakersRaw, config)

	genesisConfig := BuildGenesisConfig(allocations, now, stakersRaw[:config.NumInitialStakers], config.NetworkName)

	return &Network{
		pkg.Commit,
		genesisConfig, stakersRaw,
	}, nil
}

func BuildGenesisConfig(allocations []genesis.UnparsedAllocation, startime uint64, stakers []Staker, networkName string) genesis.UnparsedConfig {
	initialStakedFunds := make([]string, len(stakers))
	initialStakers := make([]genesis.UnparsedStaker, len(stakers))
	for i, s := range stakers {
		initialStakedFunds[i] = s.PublicAddress
		initialStakers[i] = genesis.UnparsedStaker{
			NodeID:        s.NodeID,
			RewardAddress: s.PublicAddress,
		}
	}

	return genesis.UnparsedConfig{
		NetworkID:                  1002,
		Allocations:                allocations,
		StartTime:                  startime,
		InitialStakeDuration:       31536000,
		InitialStakeDurationOffset: 5400,
		InitialStakedFunds:         initialStakedFunds,
		InitialStakers:             initialStakers,
		CChainGenesis:              "{\"config\":{\"chainId\":502,\"homesteadBlock\":0,\"daoForkBlock\":0,\"daoForkSupport\":true,\"eip150Block\":0,\"eip150Hash\":\"0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0\",\"eip155Block\":0,\"eip158Block\":0,\"byzantiumBlock\":0,\"constantinopleBlock\":0,\"petersburgBlock\":0,\"istanbulBlock\":0,\"muirGlacierBlock\":0,\"apricotPhase1BlockTimestamp\":0,\"apricotPhase2BlockTimestamp\":0,\"apricotPhase3BlockTimestamp\":0,\"apricotPhase4BlockTimestamp\":0,\"apricotPhase5BlockTimestamp\":0},\"nonce\":\"0x0\",\"timestamp\":\"0x0\",\"extraData\":\"0x00\",\"gasLimit\":\"0x5f5e100\",\"difficulty\":\"0x0\",\"mixHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"coinbase\":\"0x0000000000000000000000000000000000000000\",\"alloc\":{\"0100000000000000000000000000000000000000\":{\"code\":\"0x7300000000000000000000000000000000000000003014608060405260043610603d5760003560e01c80631e010439146042578063b6510bb314606e575b600080fd5b605c60048036036020811015605657600080fd5b503560b1565b60408051918252519081900360200190f35b818015607957600080fd5b5060af60048036036080811015608e57600080fd5b506001600160a01b03813516906020810135906040810135906060013560b6565b005b30cd90565b836001600160a01b031681836108fc8690811502906040516000604051808303818888878c8acf9550505050505015801560f4573d6000803e3d6000fd5b505050505056fea26469706673582212201eebce970fe3f5cb96bf8ac6ba5f5c133fc2908ae3dcd51082cfee8f583429d064736f6c634300060a0033\",\"balance\":\"0x0\"}},\"number\":\"0x0\",\"gasUsed\":\"0x0\",\"parentHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\"}",
		Message:                    networkName,
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
