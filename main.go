package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	dataContractABI = `[{
	"inputs": [{"internalType":"bytes32","name":"poolId","type":"bytes32"}],
	"name":"getSlot0",
	"outputs": [
		{"internalType":"uint160","name":"sqrtPriceX96","type":"uint160"},
		{"internalType":"int24","name":"tick","type":"int24"},
		{"internalType":"uint24","name":"protocolFee","type":"uint24"},
		{"internalType":"uint24","name":"lpFee","type":"uint24"}
	],
	"stateMutability":"view",
	"type":"function"
}]`

	poolManagerABI = `[{
		"anonymous":false,
		"inputs":[
			{"indexed":true,"name":"id","type":"bytes32"},
			{"indexed":true,"name":"currency0","type":"address"},
			{"indexed":true,"name":"currency1","type":"address"},
			{"indexed":false,"name":"fee","type":"uint24"},
			{"indexed":false,"name":"tickSpacing","type":"int24"},
			{"indexed":false,"name":"hooks","type":"address"},
			{"indexed":false,"name":"sqrtPriceX96","type":"uint160"},
			{"indexed":false,"name":"tick","type":"int24"}
		],
		"name":"Initialize",
		"type":"event"
	}]`
)

type InitializeEvent struct {
	PoolId       [32]byte
	Currency0    common.Address
	Currency1    common.Address
	Fee          *big.Int
	TickSpacing  *big.Int
	Hooks        common.Address
	SqrtPriceX96 *big.Int
	Tick         *big.Int
}

type Slot0Result struct {
	SqrtPriceX96 *big.Int `abi:"sqrtPriceX96"`
	Tick         *big.Int `abi:"tick"`
	ProtocolFee  *big.Int `abi:"protocolFee"`
	LpFee        *big.Int `abi:"lpFee"`
}

const (
	poolManagerAddress = "0x360E68faCcca8cA495c1B759Fd9EEe466db9FB32" // PoolManager https://docs.uniswap.org/contracts/v4/guides/read-pool-state
	dataContractAddr   = "0x76fd297e2d437cd7f76d50f01afe6160f86e9990" // StateView https://docs.uniswap.org/contracts/v4/guides/state-view
	rpcURL             = "https://arb1.arbitrum.io/rpc"
)

func main() {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatal(err)
	}

	// Инициализируем ABI для двух контрактов
	poolManagerABI, err := abi.JSON(strings.NewReader(poolManagerABI))
	if err != nil {
		log.Fatal(err)
	}

	// Получаем события Initialize
	logs, err := client.FilterLogs(context.Background(), ethereum.FilterQuery{
		Addresses: []common.Address{common.HexToAddress(poolManagerAddress)},
		Topics: [][]common.Hash{{
			poolManagerABI.Events["Initialize"].ID,
		}},
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, vLog := range logs {
		event := new(InitializeEvent)
		if err := poolManagerABI.UnpackIntoInterface(event, "Initialize", vLog.Data); err != nil {
			log.Printf("Error unpacking event: %v", err)
			continue
		}

		event.PoolId = [32]byte(vLog.Topics[1].Bytes())
		event.Currency0 = common.BytesToAddress(vLog.Topics[2].Bytes())
		event.Currency1 = common.BytesToAddress(vLog.Topics[3].Bytes())

		slot0, err := getSlot0(client, event.PoolId)
		if err != nil {
			log.Printf("Error getting slot0: %v", err)
			continue
		}

		fmt.Printf("Pool ID: 0x%x\n", event.PoolId)
		fmt.Printf("Currency0: %s\n", event.Currency0.Hex())
		fmt.Printf("Currency1: %s\n", event.Currency1.Hex())
		fmt.Printf("Fee: %d\n", event.Fee)
		fmt.Printf("Tick Spacing: %d\n", event.TickSpacing)
		fmt.Printf("Initial sqrtPriceX96: %s\n", event.SqrtPriceX96.String())
		fmt.Printf("Hooks: %s\n", event.Hooks.Hex())

		fmt.Printf("Initial tick: %d\n", event.Tick)

		fmt.Printf("\nCurrent Slot0 Data:\n")
		fmt.Printf("sqrtPriceX96: %s\n", slot0.SqrtPriceX96.String())
		fmt.Printf("Current tick: %d\n", slot0.Tick)
		fmt.Printf("Protocol Fee: %d\n", slot0.ProtocolFee)
		fmt.Printf("LP Fee: %d\n", slot0.LpFee)

		fmt.Println("----------------------------------------")
	}

	fmt.Println("Total events processed:", len(logs))
}

func getSlot0(client *ethclient.Client, poolId [32]byte) (Slot0Result, error) {
	dataContractABI, err := abi.JSON(strings.NewReader(dataContractABI))
	if err != nil {
		log.Fatal(err)
	}

	dataContract := common.HexToAddress(dataContractAddr)

	data, err := dataContractABI.Pack("getSlot0", poolId)
	if err != nil {
		return Slot0Result{}, err
	}

	result, err := client.CallContract(context.Background(), ethereum.CallMsg{To: &dataContract, Data: data}, nil)
	if err != nil {
		return Slot0Result{}, err
	}

	var out Slot0Result

	if err := dataContractABI.UnpackIntoInterface(&out, "getSlot0", result); err != nil {
		return Slot0Result{}, err
	}

	return out, nil
}
