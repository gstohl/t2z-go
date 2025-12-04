// Example 7: Mixed Transparent and Shielded Outputs (T→T+Z)
//
// Demonstrates sending to both transparent and shielded recipients:
// - Use transparent UTXOs as input
// - Send to one transparent address AND one shielded (Orchard) address
// - Shows how the library handles mixed output types in a single transaction
//
// This is a common real-world scenario where you want to pay someone
// transparently while also shielding some funds.
//
// IMPORTANT: Regtest cannot verify shielded outputs (no wallet).
// This example creates and signs the transaction but does NOT broadcast it.
//
// Run with: go run ./7-mixed-outputs

package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	t2z "github.com/gstohl/t2z/go"
	"github.com/gstohl/t2z/go/examples/zebrad-regtest/common"
)

// Deterministic mainnet unified address with Orchard receiver
// Generated from SpendingKey::from_bytes([42u8; 32])
const shieldedAddress = "u1eq7cm60un363n2sa862w4t5pq56tl5x0d7wqkzhhva0sxue7kqw85haa6w6xsz8n8ujmcpkzsza8knwgglau443s7ljdgu897yrvyhhz"

func main() {
	fmt.Println()
	fmt.Println("======================================================================")
	fmt.Println("  EXAMPLE 7: MIXED TRANSPARENT + SHIELDED OUTPUTS (T->T+Z)")
	fmt.Println("======================================================================")
	fmt.Println()

	// Set data directory relative to this script
	exe, _ := os.Executable()
	dataDir := filepath.Join(filepath.Dir(exe), "..", "data")
	common.SetDataDir(dataDir)

	// Create Zebra client
	client := common.NewZebraClient()

	// Load test data
	testData, err := common.LoadTestData()
	if err != nil {
		common.PrintError("Failed to load test data", err)
		fmt.Println("Please run setup first: go run ./setup")
		os.Exit(1)
	}

	// Transparent recipient address
	transparentRecipient := testData.Transparent.Address

	fmt.Println("Configuration:")
	fmt.Printf("  Source address: %s\n", testData.Transparent.Address)
	fmt.Printf("  Recipient 1 (transparent): %s\n", transparentRecipient)
	fmt.Printf("  Recipient 2 (shielded): %s...\n", shieldedAddress[:30])
	fmt.Println("  Note: Mixed output types in single transaction")
	fmt.Println()

	// Fetch mature coinbase UTXOs
	fmt.Println("Fetching mature coinbase UTXOs...")
	utxos, err := common.GetMatureCoinbaseUtxos(client, common.TEST_KEYPAIR, 6)
	if err != nil {
		common.PrintError("Failed to get UTXOs", err)
		os.Exit(1)
	}

	if len(utxos) < 5 {
		common.PrintError("Insufficient UTXOs", fmt.Errorf("need at least 5 mature UTXOs, got %d", len(utxos)))
		os.Exit(1)
	}

	inputs := utxos[:5]
	var totalInput uint64
	for _, u := range inputs {
		totalInput += u.Amount
	}
	fmt.Printf("  Selected %d UTXOs totaling: %s ZEC\n\n", len(inputs), common.ZatoshiToZec(totalInput))

	// Create mixed payments
	// Calculate fee: inputs, 2 transparent (1 payment + 1 change), 1 orchard
	fee := t2z.CalculateFee(len(inputs), 2, 1)
	availableForPayments := totalInput - fee

	// Split: 35% to transparent, 35% to shielded, 30% to change
	transparentPayment := availableForPayments * 35 / 100
	shieldedPayment := availableForPayments * 35 / 100

	payments := []t2z.Payment{
		{Address: transparentRecipient, Amount: transparentPayment},
		{Address: shieldedAddress, Amount: shieldedPayment},
	}

	fmt.Println("======================================================================")
	fmt.Println("  TRANSACTION SUMMARY - MIXED T+Z")
	fmt.Println("======================================================================")
	fmt.Printf("\nInput:       %s ZEC (%d UTXOs)\n", common.ZatoshiToZec(totalInput), len(inputs))
	fmt.Printf("Transparent: %s ZEC -> %s...\n", common.ZatoshiToZec(transparentPayment), transparentRecipient[:20])
	fmt.Printf("Shielded:    %s ZEC -> %s...\n", common.ZatoshiToZec(shieldedPayment), shieldedAddress[:20])
	fmt.Printf("Fee:         %s ZEC\n", common.ZatoshiToZec(fee))
	fmt.Printf("Change:      %s ZEC\n", common.ZatoshiToZec(totalInput-transparentPayment-shieldedPayment-fee))
	fmt.Println("======================================================================")
	fmt.Println()

	fmt.Println("WHAT THIS DEMONSTRATES:")
	fmt.Println("   - Single transparent input")
	fmt.Println("   - One transparent output (publicly visible)")
	fmt.Println("   - One Orchard output (shielded/private)")
	fmt.Println("   - Change returned to source address")
	fmt.Println("   - Real-world use case: pay merchant + shield savings")
	fmt.Println()

	request, err := t2z.NewTransactionRequest(payments)
	if err != nil {
		common.PrintError("Failed to create transaction request", err)
		os.Exit(1)
	}
	defer request.Free()

	// Get current block height
	info, err := client.GetBlockchainInfo()
	if err != nil {
		common.PrintError("Failed to get blockchain info", err)
		os.Exit(1)
	}
	fmt.Printf("Current block height: %d\n", info.Blocks)

	request.SetTargetHeight(2_500_000)
	fmt.Println("Using mainnet parameters (target height: 2,500,000)")
	fmt.Println()

	// Workflow
	fmt.Println("1. Proposing transaction...")
	pczt, err := t2z.ProposeTransaction(inputs, request)
	if err != nil {
		common.PrintError("Failed to propose transaction", err)
		os.Exit(1)
	}
	fmt.Println("   PCZT created with mixed outputs")
	fmt.Println()

	fmt.Println("2. Proving transaction (generating Orchard ZK proofs)...")
	fmt.Println("   This may take a few seconds...")
	proved, err := t2z.ProveTransaction(pczt)
	if err != nil {
		common.PrintError("Failed to prove transaction", err)
		os.Exit(1)
	}
	fmt.Println("   Proofs generated!")
	fmt.Println()

	fmt.Println("3. Verifying PCZT...")
	err = t2z.VerifyBeforeSigning(proved, request, []t2z.TransparentOutput{})
	if err != nil {
		fmt.Printf("   Note: Verification: %v\n", err)
	} else {
		fmt.Println("   Verification passed")
	}
	fmt.Println()

	// Sign each input
	fmt.Println("4. Signing each input...")
	currentPczt := proved
	for i := 0; i < len(inputs); i++ {
		sighash, err := t2z.GetSighash(currentPczt, uint(i))
		if err != nil {
			common.PrintError(fmt.Sprintf("Failed to get sighash for input %d", i), err)
			os.Exit(1)
		}
		signature := common.SignCompact(sighash[:], common.TEST_KEYPAIR)
		currentPczt, err = t2z.AppendSignature(currentPczt, uint(i), signature)
		if err != nil {
			common.PrintError(fmt.Sprintf("Failed to append signature for input %d", i), err)
			os.Exit(1)
		}
		fmt.Printf("   Input %d: signed\n", i)
	}
	fmt.Println()

	fmt.Println("5. Finalizing transaction...")
	txBytes, err := t2z.FinalizeAndExtract(currentPczt)
	if err != nil {
		common.PrintError("Failed to finalize transaction", err)
		os.Exit(1)
	}
	txHex := hex.EncodeToString(txBytes)
	fmt.Printf("   Transaction finalized (%d bytes)\n", len(txBytes))
	fmt.Println("   Mixed outputs = Orchard proofs + transparent outputs")
	fmt.Println()

	fmt.Println("======================================================================")
	fmt.Println("  TRANSACTION CREATED (NOT BROADCAST)")
	fmt.Println("======================================================================")
	fmt.Printf("\nTransaction size: %d bytes\n", len(txBytes))
	fmt.Printf("Transaction hex (first 100 chars): %s...\n", txHex[:100])
	fmt.Println()
	fmt.Println("NOTE: Transactions with shielded outputs cannot be broadcast to regtest")
	fmt.Println("      (Zebra has no wallet to receive shielded funds)")
	fmt.Println("      UTXOs are still available for other examples.")
	fmt.Println()

	fmt.Printf("SUCCESS! Mixed: %s ZEC transparent + %s ZEC shielded\n\n", common.ZatoshiToZec(transparentPayment), common.ZatoshiToZec(shieldedPayment))
}
