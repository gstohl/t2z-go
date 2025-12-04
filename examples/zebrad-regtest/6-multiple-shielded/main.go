// Example 6: Multiple Shielded Outputs (T→Z×2)
//
// Demonstrates sending to multiple shielded recipients:
// - Use transparent UTXOs as input
// - Send to two different unified addresses with Orchard receivers
// - Shows how the library handles multiple Orchard actions
//
// IMPORTANT: Regtest cannot verify shielded outputs (no wallet).
// This example creates and signs the transaction but does NOT broadcast it.
//
// Run with: go run ./6-multiple-shielded

package main

import (
	"encoding/hex"
	"fmt"
	"os"

	t2z "github.com/gstohl/t2z/go"
	"github.com/gstohl/t2z/go/examples/zebrad-regtest/common"
)

// Deterministic mainnet unified addresses with Orchard receivers
// Generated from SpendingKey::from_bytes([42u8; 32]) and [43u8; 32]
const shieldedAddress1 = "u1eq7cm60un363n2sa862w4t5pq56tl5x0d7wqkzhhva0sxue7kqw85haa6w6xsz8n8ujmcpkzsza8knwgglau443s7ljdgu897yrvyhhz"

// Using same address for simplicity - in real usage these would be different recipients
const shieldedAddress2 = "u1eq7cm60un363n2sa862w4t5pq56tl5x0d7wqkzhhva0sxue7kqw85haa6w6xsz8n8ujmcpkzsza8knwgglau443s7ljdgu897yrvyhhz"

func main() {
	fmt.Println()
	fmt.Println("======================================================================")
	fmt.Println("  EXAMPLE 6: MULTIPLE SHIELDED OUTPUTS (T->Z x2)")
	fmt.Println("======================================================================")
	fmt.Println()

	// Initialize data directory (respects T2Z_DATA_DIR env var)
	common.InitDataDir()

	// Create Zebra client
	client := common.NewZebraClient()

	// Load test data
	testData, err := common.LoadTestData()
	if err != nil {
		common.PrintError("Failed to load test data", err)
		fmt.Println("Please run setup first: go run ./setup")
		os.Exit(1)
	}

	fmt.Println("Configuration:")
	fmt.Printf("  Source address: %s\n", testData.Transparent.Address)
	fmt.Printf("  Recipient 1 (shielded): %s...\n", shieldedAddress1[:25])
	fmt.Printf("  Recipient 2 (shielded): %s...\n", shieldedAddress2[:25])
	fmt.Println("  Note: Both are Orchard addresses (u1... prefix)")
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

	// Create two shielded payments
	// Calculate fee: inputs, 1 transparent change, 2 orchard outputs
	fee := t2z.CalculateFee(len(inputs), 1, 2)
	availableForPayments := totalInput - fee
	payment1Amount := availableForPayments / 3 // ~33%
	payment2Amount := availableForPayments / 3 // ~33%
	// Remaining ~33% goes to change

	payments := []t2z.Payment{
		{Address: shieldedAddress1, Amount: payment1Amount},
		{Address: shieldedAddress2, Amount: payment2Amount},
	}

	fmt.Println("======================================================================")
	fmt.Println("  TRANSACTION SUMMARY - MULTIPLE SHIELDED")
	fmt.Println("======================================================================")
	fmt.Printf("\nInput:    %s ZEC (%d UTXOs)\n", common.ZatoshiToZec(totalInput), len(inputs))
	fmt.Printf("Output 1: %s ZEC -> %s...\n", common.ZatoshiToZec(payment1Amount), shieldedAddress1[:20])
	fmt.Printf("Output 2: %s ZEC -> %s...\n", common.ZatoshiToZec(payment2Amount), shieldedAddress2[:20])
	fmt.Printf("Fee:      %s ZEC\n", common.ZatoshiToZec(fee))
	fmt.Printf("Change:   %s ZEC\n", common.ZatoshiToZec(totalInput-payment1Amount-payment2Amount-fee))
	fmt.Println("======================================================================")
	fmt.Println()

	fmt.Println("WHAT THIS DEMONSTRATES:")
	fmt.Println("   - Single transparent input")
	fmt.Println("   - Two Orchard outputs (shielded recipients)")
	fmt.Println("   - Library creates multiple Orchard actions")
	fmt.Println("   - Each recipient receives private funds")
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
	fmt.Println("   PCZT created with multiple Orchard outputs")
	fmt.Println()

	fmt.Println("2. Proving transaction (generating Orchard ZK proofs)...")
	fmt.Println("   This takes longer with multiple outputs...")
	proved, err := t2z.ProveTransaction(pczt)
	if err != nil {
		common.PrintError("Failed to prove transaction", err)
		os.Exit(1)
	}
	fmt.Println("   Orchard proofs generated!")
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
	fmt.Printf("   Transaction finalized (%d bytes)\n\n", len(txBytes))

	fmt.Println("======================================================================")
	fmt.Println("  TRANSACTION CREATED (NOT BROADCAST)")
	fmt.Println("======================================================================")
	fmt.Printf("\nTransaction size: %d bytes\n", len(txBytes))
	fmt.Printf("Transaction hex (first 100 chars): %s...\n", txHex[:100])
	fmt.Println()
	fmt.Println("NOTE: Shielded transactions cannot be broadcast to regtest")
	fmt.Println("      (Zebra has no wallet to receive shielded funds)")
	fmt.Println("      UTXOs are still available for other examples.")
	fmt.Println()

	fmt.Println("SUCCESS! Shielded to 2 Orchard recipients\n")
}
