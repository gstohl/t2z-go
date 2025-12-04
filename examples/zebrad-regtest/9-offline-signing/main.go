// Example 9: Offline Signing (Hardware Wallet / Air-Gapped Device Simulation)
//
// Demonstrates the serialize/parse workflow for offline signing:
// - Online device: Creates PCZT, serializes it, outputs sighash
// - Offline device: Signs the sighash (never sees full transaction)
// - Online device: Parses PCZT, appends signature, finalizes
//
// Use case: Hardware wallets, air-gapped signing devices, or any scenario
// where the signing key never touches an internet-connected device.
//
// This example does NOT broadcast the transaction.
//
// Run with: go run ./9-offline-signing

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
	fmt.Println("  EXAMPLE 9: OFFLINE SIGNING (Hardware Wallet Simulation)")
	fmt.Println("======================================================================")
	fmt.Println()
	fmt.Println("This example demonstrates the serialize/parse workflow for offline signing.")
	fmt.Println("The private key NEVER touches the online device!")
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

	// Split keypair: public key on online device, private key on offline device
	pubkey := common.TEST_KEYPAIR.PublicKey
	privateKey := common.TEST_KEYPAIR.PrivateKey
	destAddress := testData.Transparent.Address

	fmt.Println("Configuration:")
	fmt.Printf("  Public key (online):  %s...\n", hex.EncodeToString(pubkey)[:32])
	fmt.Printf("  Private key (OFFLINE): %s...\n", hex.EncodeToString(privateKey)[:16])
	fmt.Printf("  Destination: %s\n\n", destAddress)

	// Fetch mature coinbase UTXOs
	fmt.Println("Fetching mature coinbase UTXOs...")
	utxos, err := common.GetMatureCoinbaseUtxos(client, common.TEST_KEYPAIR, 5)
	if err != nil {
		common.PrintError("Failed to get UTXOs", err)
		os.Exit(1)
	}

	if len(utxos) < 1 {
		common.PrintError("Insufficient UTXOs", fmt.Errorf("need at least 1 mature UTXO"))
		os.Exit(1)
	}

	// Use a single UTXO for simplicity
	inputs := utxos[:1]
	inputAmount := inputs[0].Amount
	fmt.Printf("  Using UTXO: %s ZEC\n\n", common.ZatoshiToZec(inputAmount))

	fee := t2z.CalculateFee(1, 2, 0)
	paymentAmount := inputAmount / 2

	payments := []t2z.Payment{{Address: destAddress, Amount: paymentAmount}}

	fmt.Println("======================================================================")
	fmt.Println("  TRANSACTION SUMMARY")
	fmt.Println("======================================================================")
	fmt.Printf("\nInput:  %s ZEC\n", common.ZatoshiToZec(inputAmount))
	fmt.Printf("Output: %s ZEC -> %s...\n", common.ZatoshiToZec(paymentAmount), destAddress[:20])
	fmt.Printf("Fee:    %s ZEC\n", common.ZatoshiToZec(fee))
	fmt.Println("======================================================================")
	fmt.Println()

	// ============================================================
	// ONLINE DEVICE: Build transaction, extract sighash
	// ============================================================
	fmt.Println("======================================================================")
	fmt.Println("  ONLINE DEVICE - Transaction Builder")
	fmt.Println("======================================================================")
	fmt.Println()

	request, err := t2z.NewTransactionRequest(payments)
	if err != nil {
		common.PrintError("Failed to create request", err)
		os.Exit(1)
	}
	defer request.Free()
	request.SetTargetHeight(2_500_000)

	fmt.Println("1. Proposing transaction...")
	pczt, err := t2z.ProposeTransaction(inputs, request)
	if err != nil {
		common.PrintError("Failed to propose", err)
		os.Exit(1)
	}
	fmt.Println("   PCZT created")

	fmt.Println("\n2. Proving transaction...")
	proved, err := t2z.ProveTransaction(pczt)
	if err != nil {
		common.PrintError("Failed to prove", err)
		os.Exit(1)
	}
	fmt.Println("   Proofs generated")

	fmt.Println("\n3. Serializing PCZT for storage...")
	pcztBytes, err := t2z.SerializePCZT(proved)
	if err != nil {
		common.PrintError("Failed to serialize", err)
		os.Exit(1)
	}
	fmt.Printf("   PCZT serialized: %d bytes\n", len(pcztBytes))

	fmt.Println("\n4. Getting sighash for offline signing...")
	sighash, err := t2z.GetSighash(proved, 0)
	if err != nil {
		common.PrintError("Failed to get sighash", err)
		os.Exit(1)
	}
	sighashHex := hex.EncodeToString(sighash[:])
	fmt.Printf("   Sighash: %s\n", sighashHex)

	fmt.Println("\n   >>> Transfer this sighash to the OFFLINE device <<<")
	fmt.Println()

	// ============================================================
	// OFFLINE DEVICE: Sign the sighash (air-gapped)
	// ============================================================
	fmt.Println("======================================================================")
	fmt.Println("  OFFLINE DEVICE - Air-Gapped Signer")
	fmt.Println("======================================================================")
	fmt.Println()

	fmt.Println("1. Receiving sighash...")
	fmt.Printf("   Sighash: %s\n", sighashHex)

	fmt.Println("\n2. Signing with private key (NEVER leaves this device)...")
	signature := common.SignCompact(sighash[:], common.TEST_KEYPAIR)
	signatureHex := hex.EncodeToString(signature[:])
	fmt.Printf("   Signature: %s\n", signatureHex)

	fmt.Println("\n   >>> Transfer this signature back to the ONLINE device <<<")
	fmt.Println()

	// ============================================================
	// ONLINE DEVICE: Append signature and finalize
	// ============================================================
	fmt.Println("======================================================================")
	fmt.Println("  ONLINE DEVICE - Finalization")
	fmt.Println("======================================================================")
	fmt.Println()

	fmt.Println("1. Parsing stored PCZT...")
	loadedPczt, err := t2z.ParsePCZT(pcztBytes)
	if err != nil {
		common.PrintError("Failed to parse", err)
		os.Exit(1)
	}
	fmt.Println("   PCZT restored from bytes")

	fmt.Println("\n2. Receiving signature from offline device...")
	fmt.Printf("   Signature: %s...\n", signatureHex[:32])

	fmt.Println("\n3. Appending signature to PCZT...")
	signed, err := t2z.AppendSignature(loadedPczt, 0, signature)
	if err != nil {
		common.PrintError("Failed to append signature", err)
		os.Exit(1)
	}
	fmt.Println("   Signature appended")

	fmt.Println("\n4. Finalizing transaction...")
	txBytes, err := t2z.FinalizeAndExtract(signed)
	if err != nil {
		common.PrintError("Failed to finalize", err)
		os.Exit(1)
	}
	txHex := hex.EncodeToString(txBytes)
	fmt.Printf("   Transaction finalized (%d bytes)\n", len(txBytes))

	fmt.Println()
	fmt.Println("======================================================================")
	fmt.Println("  TRANSACTION CREATED (NOT BROADCAST)")
	fmt.Println("======================================================================")
	fmt.Println()
	fmt.Printf("Transaction size: %d bytes\n", len(txBytes))
	fmt.Printf("Transaction hex (first 100 chars): %s...\n", txHex[:100])
	fmt.Println()
	fmt.Println("Key security properties:")
	fmt.Println("  - Private key NEVER touched the online device")
	fmt.Println("  - PCZT can be serialized/parsed for transport")
	fmt.Println("  - Sighash is safe to transfer (reveals no private data)")
	fmt.Println()
	fmt.Println("NOTE: This example demonstrates the offline signing workflow.")
	fmt.Println("      UTXOs are still available for other examples.")
	fmt.Println()
}
