// Example 1: Single Output Transaction (Transparent -> Transparent)
//
// Demonstrates the basic t2z workflow using Zebra (no wallet):
// - Load UTXO and keys from setup data
// - Create a payment to another transparent address
// - Propose, sign (client-side), and broadcast the transaction
//
// Run with: go run ./1-single-output

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
	fmt.Println("  EXAMPLE 1: SINGLE OUTPUT TRANSACTION (Transparent -> Transparent)")
	fmt.Println("======================================================================")
	fmt.Println()

	// Set data directory relative to this script
	exe, _ := os.Executable()
	dataDir := filepath.Join(filepath.Dir(exe), "..", "data")
	common.SetDataDir(dataDir)

	// Create Zebra client
	client := common.NewZebraClient()

	// Load test data for address info
	testData, err := common.LoadTestData()
	if err != nil {
		common.PrintError("Failed to load test data", err)
		fmt.Println("Please run setup first: go run ./setup")
		os.Exit(1)
	}

	fmt.Println("Configuration:")
	fmt.Printf("  Source address: %s\n", testData.Transparent.Address)

	// Fetch fresh mature coinbase UTXOs from the blockchain
	// Note: Regtest coinbase rewards are very small, so we need multiple UTXOs
	fmt.Println("Fetching mature coinbase UTXOs...")
	utxos, err := common.GetMatureCoinbaseUtxos(client, common.TEST_KEYPAIR, 6)
	if err != nil {
		common.PrintError("Failed to get UTXOs", err)
		os.Exit(1)
	}

	if len(utxos) < 5 {
		common.PrintError("Insufficient UTXOs", fmt.Errorf("need at least 5 mature UTXOs, got %d. Run setup and wait for maturity", len(utxos)))
		os.Exit(1)
	}

	// Use multiple UTXOs to have enough value (each is ~2-5k zatoshis, fee is 10000)
	inputs := utxos[:5]
	var totalInput uint64
	for _, u := range inputs {
		totalInput += u.Amount
	}
	fmt.Printf("  Selected %d UTXOs totaling: %s ZEC\n\n", len(inputs), common.ZatoshiToZec(totalInput))

	// For this example, send back to ourselves (transparent -> transparent)
	destAddress := testData.Transparent.Address
	// Calculate fee: inputs, 2 outputs (1 payment + 1 change), 0 orchard
	fee := t2z.CalculateFee(len(inputs), 2, 0)
	// Use 50% of the total input value, leaving room for fee and change
	paymentAmount := totalInput / 2

	payments := []t2z.Payment{
		{
			Address: destAddress,
			Amount:  paymentAmount,
		},
	}

	fmt.Println("Creating TransactionRequest...")
	request, err := t2z.NewTransactionRequest(payments)
	if err != nil {
		common.PrintError("Failed to create transaction request", err)
		os.Exit(1)
	}
	defer request.Free()

	// Get current block height for reference
	info, err := client.GetBlockchainInfo()
	if err != nil {
		common.PrintError("Failed to get blockchain info", err)
		os.Exit(1)
	}
	fmt.Printf("  Current block height: %d\n", info.Blocks)

	// Mainnet is the default (Zebra regtest uses mainnet-like branch IDs)
	// Set target height where NU5 is active (activated at block 1,687,104)
	targetHeight := uint32(2_500_000)
	request.SetTargetHeight(targetHeight)
	fmt.Printf("  Target height set to %d (mainnet post-NU5)\n\n", targetHeight)

	// Print workflow summary
	outputSummary := []struct {
		Address string
		Amount  uint64
	}{
		{destAddress, paymentAmount},
	}
	common.PrintWorkflowSummary("TRANSACTION SUMMARY", inputs, outputSummary, fee)

	// Step 1: Propose transaction
	fmt.Println("1. Proposing transaction...")
	pczt, err := t2z.ProposeTransaction(inputs, request)
	if err != nil {
		common.PrintError("Failed to propose transaction", err)
		os.Exit(1)
	}
	fmt.Println("   PCZT created\n")

	// Step 2: Prove transaction (for transparent-only, this is minimal)
	fmt.Println("2. Proving transaction...")
	proved, err := t2z.ProveTransaction(pczt)
	if err != nil {
		common.PrintError("Failed to prove transaction", err)
		os.Exit(1)
	}
	fmt.Println("   Proofs generated\n")

	// Step 3: Verify before signing
	fmt.Println("3. Verifying PCZT before signing...")
	err = t2z.VerifyBeforeSigning(proved, request, []t2z.TransparentOutput{})
	if err != nil {
		fmt.Printf("   Note: Verification returned: %v\n", err)
	} else {
		fmt.Println("   Verification passed")
	}
	fmt.Println()

	// Step 4-6: Sign each input
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

	// Step 7: Finalize and extract transaction bytes
	fmt.Println("5. Finalizing transaction...")
	txBytes, err := t2z.FinalizeAndExtract(currentPczt)
	if err != nil {
		common.PrintError("Failed to finalize transaction", err)
		os.Exit(1)
	}
	txHex := hex.EncodeToString(txBytes)
	fmt.Printf("   Transaction finalized (%d bytes)\n\n", len(txBytes))

	// Step 8: Broadcast transaction
	fmt.Println("6. Broadcasting transaction to network...")
	txid, err := client.SendRawTransaction(txHex)
	if err != nil {
		common.PrintError("Failed to broadcast transaction", err)
		os.Exit(1)
	}
	common.PrintBroadcastResult(txid, txHex)

	// Mark UTXOs as spent for subsequent examples
	if err := common.MarkUtxosSpent(inputs); err != nil {
		fmt.Printf("Warning: Failed to mark UTXOs as spent: %v\n", err)
	}

	// Wait for confirmation
	fmt.Println("Waiting for confirmation...")
	currentHeight := info.Blocks
	_, err = client.WaitForBlocks(currentHeight+1, 60000)
	if err != nil {
		fmt.Printf("Warning: %v\n", err)
	} else {
		fmt.Println("   Confirmed!")
	}
	fmt.Println()

	fmt.Printf("SUCCESS! TXID: %s\n\n", txid)
}
