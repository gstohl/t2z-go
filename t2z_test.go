package t2z

import (
	"encoding/hex"
	"testing"
)

// Test creating a transaction request
func TestNewTransactionRequest(t *testing.T) {
	payments := []Payment{
		{
			Address: "tm9iMLAuYMzJ6jtFLcA7rzUmfreGuKvr7Ma",
			Amount:  100_000, // 0.001 ZEC
		},
	}

	req, err := NewTransactionRequest(payments)
	if err != nil {
		t.Fatalf("Failed to create transaction request: %v", err)
	}
	defer req.Free()

	if len(req.Payments) != 1 {
		t.Errorf("Expected 1 payment, got %d", len(req.Payments))
	}

	if req.Payments[0].Amount != 100_000 {
		t.Errorf("Expected amount 100_000, got %d", req.Payments[0].Amount)
	}
}

// Test creating transaction request with multiple payments
func TestNewTransactionRequestMultiple(t *testing.T) {
	payments := []Payment{
		{
			Address: "tm9iMLAuYMzJ6jtFLcA7rzUmfreGuKvr7Ma",
			Amount:  100_000,
		},
		{
			Address: "tmBsTi2xWTjUdEXnuTceL7fecEQKeWi4vxA",
			Amount:  200_000,
		},
	}

	req, err := NewTransactionRequest(payments)
	if err != nil {
		t.Fatalf("Failed to create transaction request: %v", err)
	}
	defer req.Free()

	if len(req.Payments) != 2 {
		t.Errorf("Expected 2 payments, got %d", len(req.Payments))
	}
}

// Test creating transaction request with memo
func TestNewTransactionRequestWithMemo(t *testing.T) {
	payments := []Payment{
		{
			Address: "tm9iMLAuYMzJ6jtFLcA7rzUmfreGuKvr7Ma",
			Amount:  100_000,
			Memo:    "Test payment",
		},
	}

	req, err := NewTransactionRequest(payments)
	if err != nil {
		t.Fatalf("Failed to create transaction request: %v", err)
	}
	defer req.Free()

	if req.Payments[0].Memo != "Test payment" {
		t.Errorf("Expected memo 'Test payment', got '%s'", req.Payments[0].Memo)
	}
}

// Test empty payments error
func TestNewTransactionRequestEmpty(t *testing.T) {
	payments := []Payment{}

	_, err := NewTransactionRequest(payments)
	if err == nil {
		t.Fatal("Expected error for empty payments, got nil")
	}
}

// Test SetTargetHeight method
func TestSetTargetHeight(t *testing.T) {
	payments := []Payment{
		{
			Address: "tm9iMLAuYMzJ6jtFLcA7rzUmfreGuKvr7Ma",
			Amount:  100_000,
		},
	}

	req, err := NewTransactionRequest(payments)
	if err != nil {
		t.Fatalf("Failed to create transaction request: %v", err)
	}
	defer req.Free()

	// Set target height (post-NU5 mainnet height, like TypeScript examples)
	err = req.SetTargetHeight(2_500_000)
	if err != nil {
		t.Fatalf("Failed to set target height: %v", err)
	}
}

// Test SetUseMainnet method (matches TypeScript example behavior)
func TestSetUseMainnet(t *testing.T) {
	payments := []Payment{
		{
			Address: "tm9iMLAuYMzJ6jtFLcA7rzUmfreGuKvr7Ma",
			Amount:  100_000,
		},
	}

	req, err := NewTransactionRequest(payments)
	if err != nil {
		t.Fatalf("Failed to create transaction request: %v", err)
	}
	defer req.Free()

	// Set to use mainnet parameters (like TypeScript zebrad examples)
	err = req.SetUseMainnet(true)
	if err != nil {
		t.Fatalf("Failed to set use mainnet: %v", err)
	}

	// Also test setting it to false
	err = req.SetUseMainnet(false)
	if err != nil {
		t.Fatalf("Failed to set use mainnet to false: %v", err)
	}
}

// Test NewTransactionRequestWithTargetHeight
func TestNewTransactionRequestWithTargetHeight(t *testing.T) {
	payments := []Payment{
		{
			Address: "tm9iMLAuYMzJ6jtFLcA7rzUmfreGuKvr7Ma",
			Amount:  100_000,
		},
	}

	req, err := NewTransactionRequestWithTargetHeight(payments, 2_000_000)
	if err != nil {
		t.Fatalf("Failed to create transaction request with target height: %v", err)
	}
	defer req.Free()

	if len(req.Payments) != 1 {
		t.Errorf("Expected 1 payment, got %d", len(req.Payments))
	}
}

// Test transparent input serialization
func TestSerializeTransparentInputs(t *testing.T) {
	// Create a test pubkey (33 bytes, compressed secp256k1)
	pubkey, _ := hex.DecodeString("0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798")

	// Create a test txid (32 bytes)
	var txid [32]byte
	copy(txid[:], []byte("0000000000000000000000000000test"))

	// Create a test script_pubkey (P2PKH)
	scriptPubKey, _ := hex.DecodeString("76a914000000000000000000000000000000000000000088ac")

	inputs := []TransparentInput{
		{
			Pubkey:       pubkey,
			TxID:         txid,
			Vout:         0,
			Amount:       100_000_000, // 1 ZEC
			ScriptPubKey: scriptPubKey,
		},
	}

	serialized := serializeTransparentInputs(inputs)

	// Verify format:
	// - First 2 bytes should be number of inputs (1) in LE
	if serialized[0] != 1 || serialized[1] != 0 {
		t.Errorf("Expected num_inputs [1, 0], got [%d, %d]", serialized[0], serialized[1])
	}

	// - Next 33 bytes should be the pubkey
	if string(serialized[2:35]) != string(pubkey) {
		t.Error("Pubkey mismatch in serialization")
	}

	// - Next 32 bytes should be the txid
	if string(serialized[35:67]) != string(txid[:]) {
		t.Error("TxID mismatch in serialization")
	}

	// - Next 4 bytes should be vout (0) in LE
	if serialized[67] != 0 || serialized[68] != 0 || serialized[69] != 0 || serialized[70] != 0 {
		t.Error("Vout mismatch in serialization")
	}

	// - Next 8 bytes should be amount (100_000_000) in LE
	expectedAmount := []byte{0x00, 0xe1, 0xf5, 0x05, 0x00, 0x00, 0x00, 0x00}
	if string(serialized[71:79]) != string(expectedAmount) {
		t.Errorf("Amount mismatch: expected %v, got %v", expectedAmount, serialized[71:79])
	}

	// - Next 2 bytes should be script length (25) in LE
	if serialized[79] != 25 || serialized[80] != 0 {
		t.Errorf("Script length mismatch: expected [25, 0], got [%d, %d]", serialized[79], serialized[80])
	}

	// - Final bytes should be the script
	if string(serialized[81:]) != string(scriptPubKey) {
		t.Error("ScriptPubKey mismatch in serialization")
	}
}

// Test PCZT serialization round-trip (requires Rust library)
// This is more of an integration test
// func TestPCZTSerializeRoundtrip(t *testing.T) {
// 	// This would require actually creating a PCZT which needs the full workflow
// 	// Skip for now - will add once we have the full integration test
// 	t.Skip("Requires full PCZT creation workflow")
// }
