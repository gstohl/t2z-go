package t2z

import (
	"encoding/hex"
	"fmt"
	"log"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

// ExampleNewTransactionRequest demonstrates creating a payment request.
// Follows TypeScript zebrad-t2z example patterns.
func ExampleNewTransactionRequest() {
	// TypeScript: Uses 50% of input for payment
	// Simulating: 1 ZEC input -> 0.5 ZEC payment (50%)
	inputAmount := uint64(100_000_000) // 1 ZEC
	paymentAmount := inputAmount / 2    // 50%, like TypeScript Example 1

	payments := []Payment{
		{
			Address: "tm9iMLAuYMzJ6jtFLcA7rzUmfreGuKvr7Ma",
			Amount:  paymentAmount,
			Memo:    "Example payment",
		},
	}

	request, err := NewTransactionRequest(payments)
	if err != nil {
		log.Fatal(err)
	}
	defer request.Free()

	// Mainnet is the default
	request.SetTargetHeight(2_500_000)

	fmt.Printf("Created request with %d payment(s)\n", len(request.Payments))
	// Output: Created request with 1 payment(s)
}

// ExampleProposeTransaction demonstrates creating a PCZT from inputs and a payment request.
// Follows TypeScript zebrad-t2z Example 1 pattern.
func ExampleProposeTransaction() {
	// Match TypeScript amounts: 1 ZEC input, 50% payment
	inputAmount := uint64(100_000_000) // 1 ZEC
	paymentAmount := inputAmount / 2   // 50%

	payments := []Payment{
		{Address: "tm9iMLAuYMzJ6jtFLcA7rzUmfreGuKvr7Ma", Amount: paymentAmount},
	}
	request, _ := NewTransactionRequest(payments)
	defer request.Free()

	// Mainnet is the default
	request.SetTargetHeight(2_500_000)

	// Setup test input
	privateKeyBytes := make([]byte, 32)
	for i := range privateKeyBytes {
		privateKeyBytes[i] = 1
	}
	privKey := secp256k1.PrivKeyFromBytes(privateKeyBytes)
	pubKeyBytes := privKey.PubKey().SerializeCompressed()

	scriptPubKeyHex := "76a91479b000887626b294a914501a4cd226b58b23598388ac"
	scriptPubKey, _ := hex.DecodeString(scriptPubKeyHex)

	var txid [32]byte
	inputs := []TransparentInput{
		{
			Pubkey:       pubKeyBytes,
			TxID:         txid,
			Vout:         0,
			Amount:       inputAmount,
			ScriptPubKey: scriptPubKey,
		},
	}

	// Propose transaction (creates PCZT)
	_, err := ProposeTransaction(inputs, request)
	if err != nil {
		log.Fatal(err)
	}
	// Note: pczt is consumed by next operations, don't Free() it

	fmt.Println("PCZT created successfully")
	// Output: PCZT created successfully
}

// ExampleSerializePCZT demonstrates serializing a PCZT for transmission or storage.
// Follows TypeScript patterns.
func ExampleSerializePCZT() {
	// Match TypeScript amounts
	inputAmount := uint64(100_000_000) // 1 ZEC
	paymentAmount := inputAmount / 2   // 50%

	payments := []Payment{
		{Address: "tm9iMLAuYMzJ6jtFLcA7rzUmfreGuKvr7Ma", Amount: paymentAmount},
	}
	request, _ := NewTransactionRequest(payments)
	defer request.Free()

	// Mainnet is the default
	request.SetTargetHeight(2_500_000)

	privateKeyBytes := make([]byte, 32)
	for i := range privateKeyBytes {
		privateKeyBytes[i] = 1
	}
	privKey := secp256k1.PrivKeyFromBytes(privateKeyBytes)
	pubKeyBytes := privKey.PubKey().SerializeCompressed()
	scriptPubKey, _ := hex.DecodeString("76a91479b000887626b294a914501a4cd226b58b23598388ac")

	var txid [32]byte
	inputs := []TransparentInput{
		{Pubkey: pubKeyBytes, TxID: txid, Vout: 0, Amount: inputAmount, ScriptPubKey: scriptPubKey},
	}

	pczt, _ := ProposeTransaction(inputs, request)

	// Serialize PCZT (does not consume it)
	pcztBytes, err := SerializePCZT(pczt)
	if err != nil {
		log.Fatal(err)
	}

	// Free after serialization
	pczt.Free()

	fmt.Printf("Serialized PCZT: %d bytes\n", len(pcztBytes))
	// Output: Serialized PCZT: 367 bytes
}

// ExampleParsePCZT demonstrates parsing a serialized PCZT.
// Follows TypeScript patterns.
func ExampleParsePCZT() {
	// Assume we have serialized PCZT bytes (e.g., from hardware wallet)
	// This would come from SerializePCZT() in production
	pcztBytesHex := "..." // Placeholder

	pcztBytes, _ := hex.DecodeString(pcztBytesHex)
	if len(pcztBytes) == 0 {
		// For example purposes, create valid bytes
		// Match TypeScript amounts
		inputAmount := uint64(100_000_000)
		paymentAmount := inputAmount / 2

		payments := []Payment{
			{Address: "tm9iMLAuYMzJ6jtFLcA7rzUmfreGuKvr7Ma", Amount: paymentAmount},
		}
		request, _ := NewTransactionRequest(payments)
		defer request.Free()

		// Mainnet is the default
		request.SetTargetHeight(2_500_000)

		privateKeyBytes := make([]byte, 32)
		for i := range privateKeyBytes {
			privateKeyBytes[i] = 1
		}
		privKey := secp256k1.PrivKeyFromBytes(privateKeyBytes)
		pubKeyBytes := privKey.PubKey().SerializeCompressed()
		scriptPubKey, _ := hex.DecodeString("76a91479b000887626b294a914501a4cd226b58b23598388ac")

		var txid [32]byte
		inputs := []TransparentInput{
			{Pubkey: pubKeyBytes, TxID: txid, Vout: 0, Amount: inputAmount, ScriptPubKey: scriptPubKey},
		}

		pczt, _ := ProposeTransaction(inputs, request)
		pcztBytes, _ = SerializePCZT(pczt)
		pczt.Free()
	}

	// Parse PCZT from bytes
	pczt, err := ParsePCZT(pcztBytes)
	if err != nil {
		log.Fatal(err)
	}
	defer pczt.Free()

	fmt.Println("PCZT parsed successfully")
	// Output: PCZT parsed successfully
}

// ExampleGetSighash demonstrates getting a signature hash for signing.
// Follows TypeScript patterns.
func ExampleGetSighash() {
	// Match TypeScript amounts
	inputAmount := uint64(100_000_000) // 1 ZEC
	paymentAmount := inputAmount / 2   // 50%

	payments := []Payment{
		{Address: "tm9iMLAuYMzJ6jtFLcA7rzUmfreGuKvr7Ma", Amount: paymentAmount},
	}
	request, _ := NewTransactionRequest(payments)
	defer request.Free()

	// Mainnet is the default
	request.SetTargetHeight(2_500_000)

	privateKeyBytes := make([]byte, 32)
	for i := range privateKeyBytes {
		privateKeyBytes[i] = 1
	}
	privKey := secp256k1.PrivKeyFromBytes(privateKeyBytes)
	pubKeyBytes := privKey.PubKey().SerializeCompressed()
	scriptPubKey, _ := hex.DecodeString("76a91479b000887626b294a914501a4cd226b58b23598388ac")

	var txid [32]byte
	inputs := []TransparentInput{
		{Pubkey: pubKeyBytes, TxID: txid, Vout: 0, Amount: inputAmount, ScriptPubKey: scriptPubKey},
	}

	pczt, _ := ProposeTransaction(inputs, request)
	proved, _ := ProveTransaction(pczt)

	// Get sighash for input 0 (does not consume PCZT)
	sighash, err := GetSighash(proved, 0)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Sighash length: %d bytes\n", len(sighash))
	// Output: Sighash length: 32 bytes
}

// ExampleNewTransactionRequestWithTargetHeight demonstrates creating a request with target height.
// Matches TypeScript zebrad example target height.
func ExampleNewTransactionRequestWithTargetHeight() {
	// Match TypeScript amounts
	inputAmount := uint64(100_000_000)
	paymentAmount := inputAmount / 2

	payments := []Payment{
		{
			Address: "tm9iMLAuYMzJ6jtFLcA7rzUmfreGuKvr7Ma",
			Amount:  paymentAmount,
		},
	}

	// Create request with specific target block height (post-NU5 like TypeScript)
	request, err := NewTransactionRequestWithTargetHeight(payments, 2_500_000)
	if err != nil {
		log.Fatal(err)
	}
	defer request.Free()

	// Mainnet is the default (regtest uses mainnet branch IDs)

	fmt.Printf("Created request with %d payment(s)\n", len(request.Payments))
	// Output: Created request with 1 payment(s)
}

// ExampleTransactionRequest_SetTargetHeight demonstrates setting target height on existing request.
// Matches TypeScript zebrad patterns.
func ExampleTransactionRequest_SetTargetHeight() {
	// Match TypeScript amounts
	inputAmount := uint64(100_000_000)
	paymentAmount := inputAmount / 2

	payments := []Payment{
		{Address: "tm9iMLAuYMzJ6jtFLcA7rzUmfreGuKvr7Ma", Amount: paymentAmount},
	}

	request, _ := NewTransactionRequest(payments)
	defer request.Free()

	// Set target height after creation (post-NU5)
	err := request.SetTargetHeight(2_500_000)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Target height set successfully")
	// Output: Target height set successfully
}
