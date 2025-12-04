// Setup script to initialize Zebra regtest environment
//
// This script:
// 1. Waits for Zebra to be ready
// 2. Waits for coinbase maturity (101 blocks)
// 3. Saves test keypair data for examples
//
// Run with: go run ./setup

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gstohl/t2z/go/examples/zebrad-regtest/common"
)

func main() {
	fmt.Println("Setting up Zebra regtest environment...\n")

	// Set data directory relative to this script
	exe, _ := os.Executable()
	dataDir := filepath.Join(filepath.Dir(exe), "..", "data")
	common.SetDataDir(dataDir)
	os.MkdirAll(dataDir, 0755)

	// Clear spent UTXOs tracker from previous runs
	common.ClearSpentUtxos()
	fmt.Println("Cleared spent UTXO tracker\n")

	client := common.NewZebraClient()

	// Wait for Zebra to be ready
	fmt.Println("Waiting for Zebra...")
	if err := client.WaitForReady(30, 1000); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Zebra is ready\n")

	// Get blockchain info
	info, err := client.GetBlockchainInfo()
	if err != nil {
		fmt.Printf("Error getting blockchain info: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Chain: %s\n", info.Chain)
	fmt.Printf("Current height: %d\n\n", info.Blocks)

	// Wait for coinbase maturity (101 blocks needed)
	targetHeight := 101
	if info.Blocks < targetHeight {
		fmt.Printf("Waiting for internal miner to reach height %d...\n", targetHeight)
		fmt.Println("(Zebra internal miner auto-mines blocks every ~30 seconds)\n")

		finalHeight, err := client.WaitForBlocks(targetHeight, 600000) // 10 min timeout
		if err != nil {
			fmt.Printf("Error waiting for blocks: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Reached height %d\n\n", finalHeight)
	}

	// Use our pre-defined test keypair
	keypair := common.TEST_KEYPAIR
	fmt.Println("Using test keypair from common/keys.go:")
	fmt.Printf("  Address: %s\n", keypair.Address)
	fmt.Printf("  WIF: %s\n\n", keypair.WIF)

	// Get final blockchain info
	finalInfo, err := client.GetBlockchainInfo()
	if err != nil {
		fmt.Printf("Error getting final blockchain info: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Final height: %d\n\n", finalInfo.Blocks)

	// Check for mature coinbase UTXOs
	fmt.Println("Checking for mature coinbase UTXOs...\n")
	utxos, err := common.GetMatureCoinbaseUtxos(client, keypair, 10)
	if err != nil {
		fmt.Printf("Error getting UTXOs: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d mature coinbase UTXOs for our address\n", len(utxos))
	for i, utxo := range utxos {
		txidHex := common.BytesToHex(utxo.TxID[:])
		fmt.Printf("  [%d] %s...:0 = %s ZEC\n", i, txidHex[:16], common.ZatoshiToZec(utxo.Amount))
	}

	// Save test keypair data for examples
	testData := &common.TestData{
		Transparent: common.TransparentData{
			Address:    keypair.Address,
			PublicKey:  common.BytesToHex(keypair.PublicKey),
			PrivateKey: common.BytesToHex(keypair.PrivateKey),
			WIF:        keypair.WIF,
		},
		Network:     info.Chain,
		SetupHeight: finalInfo.Blocks,
		SetupAt:     time.Now().Format(time.RFC3339),
	}

	if err := common.SaveTestData(testData); err != nil {
		fmt.Printf("Error saving test data: %v\n", err)
	} else {
		fmt.Printf("\nSaved test data to %s/test-addresses.json\n", dataDir)
	}

	fmt.Println("\nSetup complete!")
	fmt.Println("\nYou can now run the examples:")
	fmt.Println("  go run ./1-single-output      # Single transparent output (T→T)")
	fmt.Println("  go run ./2-multiple-outputs   # Multiple transparent outputs (T→T×2)")
	fmt.Println("  go run ./3-utxo-consolidation # UTXO consolidation (2 inputs → 1 output)")
	fmt.Println("  go run ./4-attack-scenario    # Attack detection - PCZT verification")
	fmt.Println("  go run ./5-shielded-output    # Single shielded output (T→Z)")
	fmt.Println("  go run ./6-multiple-shielded  # Multiple shielded outputs (T→Z×2)")
	fmt.Println("  go run ./7-mixed-outputs      # Mixed transparent + shielded (T→T+Z)")
	fmt.Println("  go run ./8-combine-workflow   # Combine workflow (parallel signing)")
	fmt.Println("  go run ./9-offline-signing    # Offline signing (hardware wallet)")
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
}
