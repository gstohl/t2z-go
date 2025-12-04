// Example 2: Multiple Output Transaction (T→T×2)
//
// Demonstrates sending to multiple transparent addresses in a single transaction:
// - Use multiple UTXOs as inputs
// - Create multiple outputs (2 recipients + change)
// - Show how change is handled
//
// Run with: go run ./2-multiple-outputs

package main

import (
	"encoding/hex"
	"fmt"
	"os"

	t2z "github.com/gstohl/t2z/go"
	"github.com/gstohl/t2z/go/examples/zebrad-regtest/common"
)

func main() {
	fmt.Println()
	fmt.Println("======================================================================")
	fmt.Println("  EXAMPLE 2: MULTIPLE OUTPUT TRANSACTION (T→T×2)")
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

	// Two destination addresses - using same address for simplicity
	dest1 := testData.Transparent.Address
	dest2 := testData.Transparent.Address

	fmt.Println("Configuration:")
	fmt.Printf("  Source address: %s\n", testData.Transparent.Address)
	fmt.Printf("  Destination 1: %s\n", dest1)
	fmt.Printf("  Destination 2: %s\n\n", dest2)

	// Fetch fresh mature coinbase UTXOs
	fmt.Println("Fetching mature coinbase UTXOs...")
	utxos, err := common.GetMatureCoinbaseUtxos(client, common.TEST_KEYPAIR, 10)
	if err != nil {
		common.PrintError("Failed to get UTXOs", err)
		os.Exit(1)
	}

	if len(utxos) < 5 {
		common.PrintError("Insufficient UTXOs", fmt.Errorf("need at least 5 mature UTXOs, got %d", len(utxos)))
		os.Exit(1)
	}

	// Use 5 UTXOs
	inputs := utxos[:5]
	var totalInput uint64
	for _, u := range inputs {
		totalInput += u.Amount
	}
	fmt.Printf("  Selected %d UTXOs totaling: %s ZEC\n\n", len(inputs), common.ZatoshiToZec(totalInput))

	// Calculate fee: inputs, 3 outputs (2 payments + 1 change), 0 orchard
	fee := t2z.CalculateFee(len(inputs), 3, 0)
	availableForPayments := totalInput - fee
	payment1Amount := availableForPayments * 30 / 100 // 30%
	payment2Amount := availableForPayments * 30 / 100 // 30%

	payments := []t2z.Payment{
		{Address: dest1, Amount: payment1Amount},
		{Address: dest2, Amount: payment2Amount},
	}

	fmt.Println("======================================================================")
	fmt.Println("  TRANSACTION SUMMARY - TWO RECIPIENTS")
	fmt.Println("======================================================================")
	fmt.Printf("\nInput:   %s ZEC (%d UTXOs)\n", common.ZatoshiToZec(totalInput), len(inputs))
	fmt.Printf("Output 1: %s ZEC -> %s...\n", common.ZatoshiToZec(payment1Amount), dest1[:20])
	fmt.Printf("Output 2: %s ZEC -> %s...\n", common.ZatoshiToZec(payment2Amount), dest2[:20])
	fmt.Printf("Fee:      %s ZEC\n", common.ZatoshiToZec(fee))
	fmt.Printf("Change:   %s ZEC\n", common.ZatoshiToZec(totalInput-payment1Amount-payment2Amount-fee))
	fmt.Println("======================================================================")
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

	// Mainnet is the default
	request.SetTargetHeight(2_500_000)
	fmt.Println("Using mainnet parameters")
	fmt.Println()

	// Workflow
	fmt.Println("1. Proposing transaction...")
	pczt, err := t2z.ProposeTransaction(inputs, request)
	if err != nil {
		common.PrintError("Failed to propose transaction", err)
		os.Exit(1)
	}
	fmt.Println("   PCZT created with 2 outputs + change")
	fmt.Println()

	fmt.Println("2. Proving transaction...")
	proved, err := t2z.ProveTransaction(pczt)
	if err != nil {
		common.PrintError("Failed to prove transaction", err)
		os.Exit(1)
	}
	fmt.Println("   Proofs generated")
	fmt.Println()

	fmt.Println("3. Verifying PCZT...")
	err = t2z.VerifyBeforeSigning(proved, request, []t2z.TransparentOutput{})
	if err != nil {
		fmt.Printf("   Note: Verification: %v\n", err)
	} else {
		fmt.Println("   Verified: both payments present")
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

	// Broadcast transaction
	fmt.Println("6. Broadcasting transaction to network...")
	txid, err := client.SendRawTransaction(txHex)
	if err != nil {
		common.PrintError("Failed to broadcast transaction", err)
		os.Exit(1)
	}
	common.PrintBroadcastResult(txid, txHex)

	// Mark UTXOs as spent
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
