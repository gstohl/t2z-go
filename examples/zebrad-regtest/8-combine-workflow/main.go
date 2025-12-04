// Example 8: Combine Workflow (Parallel Signing)
//
// Demonstrates the combine() function for multi-party signing workflows:
// - Create a transaction with multiple inputs
// - Serialize the PCZT and create copies for parallel signing
// - Each "signer" signs their input independently
// - Combine the partially-signed PCZTs into one
// - Finalize the transaction
//
// Use case: Multiple parties each control different UTXOs and need to
// co-sign a transaction without sharing private keys.
//
// This example does NOT broadcast the transaction.
//
// Run with: go run ./8-combine-workflow

package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	t2z "github.com/gstohl/t2z/go"
	"github.com/gstohl/t2z/go/examples/zebrad-regtest/common"
)

func main() {
	fmt.Println()
	fmt.Println("======================================================================")
	fmt.Println("  EXAMPLE 8: COMBINE WORKFLOW (Parallel Signing)")
	fmt.Println("======================================================================")
	fmt.Println()
	fmt.Println("This example demonstrates the Combine() function for parallel signing.")
	fmt.Println("In a real scenario, each signer would be on a different device.")
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

	destAddress := testData.Transparent.Address

	fmt.Println("Configuration:")
	fmt.Printf("  Address: %s\n", destAddress)
	fmt.Println()

	// Fetch mature coinbase UTXOs
	fmt.Println("Fetching mature coinbase UTXOs...")
	utxos, err := common.GetMatureCoinbaseUtxos(client, common.TEST_KEYPAIR, 6)
	if err != nil {
		common.PrintError("Failed to get UTXOs", err)
		os.Exit(1)
	}

	if len(utxos) < 3 {
		common.PrintError("Insufficient UTXOs", fmt.Errorf("need at least 3 mature UTXOs, got %d", len(utxos)))
		os.Exit(1)
	}

	// Use 3 UTXOs to simulate 3 different parties
	inputs := utxos[:3]
	var totalInput uint64
	for _, u := range inputs {
		totalInput += u.Amount
	}
	fmt.Printf("  Selected %d UTXOs totaling: %s ZEC\n\n", len(inputs), common.ZatoshiToZec(totalInput))

	// Create payment
	fee := t2z.CalculateFee(3, 2, 0)
	paymentAmount := totalInput / 2

	payments := []t2z.Payment{{Address: destAddress, Amount: paymentAmount}}

	fmt.Println("======================================================================")
	fmt.Println("  TRANSACTION SUMMARY")
	fmt.Println("======================================================================")
	fmt.Printf("\nInputs: 3 UTXOs = %s ZEC\n", common.ZatoshiToZec(totalInput))
	fmt.Printf("Output: %s ZEC -> %s...\n", common.ZatoshiToZec(paymentAmount), destAddress[:20])
	fmt.Printf("Fee:    %s ZEC\n", common.ZatoshiToZec(fee))
	fmt.Println("======================================================================")
	fmt.Println()

	request, err := t2z.NewTransactionRequest(payments)
	if err != nil {
		common.PrintError("Failed to create request", err)
		os.Exit(1)
	}
	defer request.Free()
	request.SetTargetHeight(2_500_000)

	fmt.Println("--- PARALLEL SIGNING WORKFLOW ---\n")

	// Step 1: Create and prove the PCZT
	fmt.Println("1. Creating and proving PCZT...")
	pczt, err := t2z.ProposeTransaction(inputs, request)
	if err != nil {
		common.PrintError("Failed to propose", err)
		os.Exit(1)
	}
	proved, err := t2z.ProveTransaction(pczt)
	if err != nil {
		common.PrintError("Failed to prove", err)
		os.Exit(1)
	}
	fmt.Println("   PCZT created and proved\n")

	// Step 2: Serialize the proved PCZT
	fmt.Println("2. Serializing PCZT for distribution to signers...")
	pcztBytes, err := t2z.SerializePCZT(proved)
	if err != nil {
		common.PrintError("Failed to serialize", err)
		os.Exit(1)
	}
	fmt.Printf("   Serialized PCZT: %d bytes\n\n", len(pcztBytes))

	// Step 3: Simulate parallel signing by different parties
	fmt.Println("3. Simulating parallel signing by 3 different parties...\n")

	// Signer A signs input 0
	fmt.Println("   Signer A: Signing input 0...")
	pcztA, _ := t2z.ParsePCZT(pcztBytes)
	sighashA, _ := t2z.GetSighash(pcztA, 0)
	signatureA := common.SignCompact(sighashA[:], common.TEST_KEYPAIR)
	signedA, _ := t2z.AppendSignature(pcztA, 0, signatureA)
	bytesA, _ := t2z.SerializePCZT(signedA)
	fmt.Println("   Signer A: Done (signed input 0)\n")

	// Signer B signs input 1
	fmt.Println("   Signer B: Signing input 1...")
	pcztB, _ := t2z.ParsePCZT(pcztBytes)
	sighashB, _ := t2z.GetSighash(pcztB, 1)
	signatureB := common.SignCompact(sighashB[:], common.TEST_KEYPAIR)
	signedB, _ := t2z.AppendSignature(pcztB, 1, signatureB)
	bytesB, _ := t2z.SerializePCZT(signedB)
	fmt.Println("   Signer B: Done (signed input 1)\n")

	// Signer C signs input 2
	fmt.Println("   Signer C: Signing input 2...")
	pcztC, _ := t2z.ParsePCZT(pcztBytes)
	sighashC, _ := t2z.GetSighash(pcztC, 2)
	signatureC := common.SignCompact(sighashC[:], common.TEST_KEYPAIR)
	signedC, _ := t2z.AppendSignature(pcztC, 2, signatureC)
	bytesC, _ := t2z.SerializePCZT(signedC)
	fmt.Println("   Signer C: Done (signed input 2)\n")

	// Step 4: Combine all partially-signed PCZTs
	fmt.Println("4. Combining partially-signed PCZTs...")
	combinedA, _ := t2z.ParsePCZT(bytesA)
	combinedB, _ := t2z.ParsePCZT(bytesB)
	combinedC, _ := t2z.ParsePCZT(bytesC)

	fullySignedPczt, err := t2z.Combine([]*t2z.PCZT{combinedA, combinedB, combinedC})
	if err != nil {
		common.PrintError("Failed to combine", err)
		os.Exit(1)
	}
	fmt.Println("   All signatures combined into single PCZT\n")

	// Step 5: Finalize
	fmt.Println("5. Finalizing transaction...")
	txBytes, err := t2z.FinalizeAndExtract(fullySignedPczt)
	if err != nil {
		common.PrintError("Failed to finalize", err)
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
	fmt.Println("NOTE: This example demonstrates the Combine() workflow.")
	fmt.Println("      UTXOs are still available for other examples.")
	fmt.Println()

	fmt.Printf("SUCCESS! Transaction ready (%d bytes)\n", len(txBytes))
	fmt.Println("The Combine() function merged signatures from 3 independent signers.\n")
}
