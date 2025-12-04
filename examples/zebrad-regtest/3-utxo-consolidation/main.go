// Example 3: Multiple Inputs Transaction (UTXO Consolidation)
//
// Demonstrates combining multiple UTXOs in a single transaction:
// - Use multiple UTXOs as inputs
// - Sign each input separately
// - Consolidate funds into fewer outputs
//
// Run with: go run ./3-utxo-consolidation

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
	fmt.Println("  EXAMPLE 3: MULTIPLE INPUTS (UTXO Consolidation)")
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

	destAddress := testData.Transparent.Address

	fmt.Println("Configuration:")
	fmt.Printf("  Address: %s\n\n", destAddress)

	// Fetch fresh mature coinbase UTXOs
	fmt.Println("Fetching mature coinbase UTXOs...")
	utxos, err := common.GetMatureCoinbaseUtxos(client, common.TEST_KEYPAIR, 10)
	if err != nil {
		common.PrintError("Failed to get UTXOs", err)
		os.Exit(1)
	}

	// For consolidation, we want multiple inputs going to single output
	minUtxos := 6
	if len(utxos) < minUtxos {
		common.PrintError("Insufficient UTXOs", fmt.Errorf("need at least %d mature UTXOs, got %d. Run more examples to create UTXOs", minUtxos, len(utxos)))
		os.Exit(1)
	}

	// Use 6 UTXOs for consolidation
	inputs := utxos[:minUtxos]
	var totalInput uint64
	for _, u := range inputs {
		totalInput += u.Amount
	}

	fmt.Printf("Found %d UTXOs:\n", len(inputs))
	for i, inp := range inputs {
		fmt.Printf("  [%d] %s ZEC\n", i, common.ZatoshiToZec(inp.Amount))
	}
	fmt.Printf("  Total: %s ZEC\n\n", common.ZatoshiToZec(totalInput))

	// Calculate fee: N inputs, 1 output (consolidation), 0 orchard
	fee := t2z.CalculateFee(len(inputs), 1, 0)
	outputAmount := totalInput - fee

	payments := []t2z.Payment{
		{Address: destAddress, Amount: outputAmount},
	}

	fmt.Println("======================================================================")
	fmt.Println("  TRANSACTION SUMMARY - UTXO CONSOLIDATION")
	fmt.Println("======================================================================")
	fmt.Printf("\nInputs:  %d UTXOs totaling %s ZEC\n", len(inputs), common.ZatoshiToZec(totalInput))
	fmt.Printf("Output:  %s ZEC (consolidated)\n", common.ZatoshiToZec(outputAmount))
	fmt.Printf("Fee:     %s ZEC\n", common.ZatoshiToZec(fee))
	fmt.Printf("Result:  %d UTXOs -> 1 UTXO\n", len(inputs))
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
	request.SetTargetHeight(2_500_000)
	fmt.Println()

	// Workflow
	fmt.Println("1. Proposing transaction with multiple inputs...")
	pczt, err := t2z.ProposeTransaction(inputs, request)
	if err != nil {
		common.PrintError("Failed to propose transaction", err)
		os.Exit(1)
	}
	fmt.Printf("   PCZT created with %d inputs\n", len(inputs))
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
		fmt.Printf("   Note: %v\n", err)
	} else {
		fmt.Println("   Verification passed")
	}
	fmt.Println()

	// Sign each input separately (key difference for multiple inputs!)
	fmt.Println("4. Getting sighashes and signing each input...")
	currentPczt := proved
	for i := range inputs {
		fmt.Printf("   Input %d:\n", i)

		sighash, err := t2z.GetSighash(currentPczt, uint(i))
		if err != nil {
			common.PrintError(fmt.Sprintf("Failed to get sighash for input %d", i), err)
			os.Exit(1)
		}
		fmt.Printf("     Sighash: %s...\n", hex.EncodeToString(sighash[:])[:24])

		signature := common.SignCompact(sighash[:], common.TEST_KEYPAIR)
		fmt.Printf("     Signature: %s...\n", hex.EncodeToString(signature[:])[:24])

		currentPczt, err = t2z.AppendSignature(currentPczt, uint(i), signature)
		if err != nil {
			common.PrintError(fmt.Sprintf("Failed to append signature for input %d", i), err)
			os.Exit(1)
		}
		fmt.Println("     Signature appended")
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

	fmt.Printf("SUCCESS! %d UTXOs consolidated into 1\n", len(inputs))
	fmt.Printf("TXID: %s\n\n", txid)
}
