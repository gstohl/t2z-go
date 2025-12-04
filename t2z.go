// Package t2z provides Go bindings for the t2z (Transparent to Zcash) library.
//
// This library enables transparent Zcash users to send shielded Orchard outputs
// using PCZT (Partially Constructed Zcash Transaction) as defined in ZIP 374.
//
// The library wraps the Rust t2z library via CGO, providing a Go-idiomatic API
// for all 8 required functions:
//   - ProposeTransaction - Creates a PCZT from inputs and payment requests
//   - ProveTransaction - Adds Orchard proofs
//   - VerifyBeforeSigning - Optional pre-signing validation
//   - GetSighash - Gets signature hash for an input
//   - AppendSignature - Adds a signature from external signing
//   - Combine - Merges multiple PCZTs
//   - FinalizeAndExtract - Produces final transaction bytes
//   - Parse/Serialize - PCZT serialization
package t2z

// #cgo CFLAGS: -I${SRCDIR}/include
// #cgo darwin,arm64 LDFLAGS: ${SRCDIR}/lib/darwin-arm64/libt2z.a -ldl -lm -framework Security -framework Foundation
// #cgo darwin,amd64 LDFLAGS: ${SRCDIR}/lib/darwin-x64/libt2z.a -ldl -lm -framework Security -framework Foundation
// #cgo linux,amd64 LDFLAGS: ${SRCDIR}/lib/linux-x64/libt2z.a -ldl -lm -lpthread
// #cgo linux,arm64 LDFLAGS: ${SRCDIR}/lib/linux-arm64/libt2z.a -ldl -lm -lpthread
// #cgo windows,amd64 LDFLAGS: ${SRCDIR}/lib/windows-x64/t2z.lib
// #cgo windows,arm64 LDFLAGS: ${SRCDIR}/lib/windows-arm64/t2z.lib
// #include <stdlib.h>
// #include "t2z.h"
import "C"
import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"
)

// ResultCode represents the result of an FFI function call
type ResultCode int

const (
	Success            ResultCode = C.SUCCESS
	ErrorNullPointer   ResultCode = C.ERROR_NULL_POINTER
	ErrorInvalidUTF8   ResultCode = C.ERROR_INVALID_UTF8
	ErrorBufferTooSmall ResultCode = C.ERROR_BUFFER_TOO_SMALL
	ErrorProposal      ResultCode = C.ERROR_PROPOSAL
	ErrorProver        ResultCode = C.ERROR_PROVER
	ErrorVerification  ResultCode = C.ERROR_VERIFICATION
	ErrorSighash       ResultCode = C.ERROR_SIGHASH
	ErrorSignature     ResultCode = C.ERROR_SIGNATURE
	ErrorCombine       ResultCode = C.ERROR_COMBINE
	ErrorFinalization  ResultCode = C.ERROR_FINALIZATION
	ErrorParse         ResultCode = C.ERROR_PARSE
	ErrorNotImplemented ResultCode = C.ERROR_NOT_IMPLEMENTED
)

// String returns the string representation of a ResultCode
func (r ResultCode) String() string {
	switch r {
	case Success:
		return "Success"
	case ErrorNullPointer:
		return "ErrorNullPointer"
	case ErrorInvalidUTF8:
		return "ErrorInvalidUTF8"
	case ErrorBufferTooSmall:
		return "ErrorBufferTooSmall"
	case ErrorProposal:
		return "ErrorProposal"
	case ErrorProver:
		return "ErrorProver"
	case ErrorVerification:
		return "ErrorVerification"
	case ErrorSighash:
		return "ErrorSighash"
	case ErrorSignature:
		return "ErrorSignature"
	case ErrorCombine:
		return "ErrorCombine"
	case ErrorFinalization:
		return "ErrorFinalization"
	case ErrorParse:
		return "ErrorParse"
	case ErrorNotImplemented:
		return "ErrorNotImplemented"
	default:
		return fmt.Sprintf("Unknown(%d)", r)
	}
}

// getLastError retrieves the last error message from the Rust library
func getLastError() string {
	buf := make([]byte, 512)
	code := C.pczt_get_last_error((*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf)))
	if code != C.SUCCESS {
		return "Failed to get last error"
	}
	// Find null terminator
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

// wrapError wraps a ResultCode into a Go error with the last error message
func wrapError(code ResultCode) error {
	if code == Success {
		return nil
	}
	msg := getLastError()
	if msg == "" {
		return fmt.Errorf("t2z error: %s", code.String())
	}
	return fmt.Errorf("t2z error: %s: %s", code.String(), msg)
}

// Payment represents a single payment to a recipient
type Payment struct {
	// Address can be a transparent address (starts with 't')
	// or a unified address with Orchard receiver (starts with 'u')
	Address string

	// Amount in zatoshis (1 ZEC = 100,000,000 zatoshis)
	Amount uint64

	// Optional memo for shielded outputs (max 512 bytes)
	Memo string

	// Optional label for the recipient
	Label string

	// Optional message
	Message string
}

// TransactionRequest represents a ZIP 321 payment request
type TransactionRequest struct {
	Payments []Payment
	handle   *C.TransactionRequestHandle
}

// NewTransactionRequest creates a new transaction request from a list of payments
func NewTransactionRequest(payments []Payment) (*TransactionRequest, error) {
	if len(payments) == 0 {
		return nil, errors.New("at least one payment is required")
	}

	// Convert payments to C array
	cPayments := make([]C.CPayment, len(payments))
	var cStrings []*C.char

	for i, payment := range payments {
		// Convert address (required)
		cAddr := C.CString(payment.Address)
		cStrings = append(cStrings, cAddr)
		cPayments[i].address = cAddr
		cPayments[i].amount = C.uint64_t(payment.Amount)

		// Convert optional fields
		if payment.Memo != "" {
			cMemo := C.CString(payment.Memo)
			cStrings = append(cStrings, cMemo)
			cPayments[i].memo = cMemo
		}
		if payment.Label != "" {
			cLabel := C.CString(payment.Label)
			cStrings = append(cStrings, cLabel)
			cPayments[i].label = cLabel
		}
		if payment.Message != "" {
			cMsg := C.CString(payment.Message)
			cStrings = append(cStrings, cMsg)
			cPayments[i].message = cMsg
		}
	}

	// Cleanup C strings when done
	defer func() {
		for _, s := range cStrings {
			C.free(unsafe.Pointer(s))
		}
	}()

	var handle *C.TransactionRequestHandle
	code := C.pczt_transaction_request_new(
		&cPayments[0],
		C.size_t(len(payments)),
		&handle,
	)

	if code != C.SUCCESS {
		return nil, wrapError(ResultCode(code))
	}

	req := &TransactionRequest{
		Payments: payments,
		handle:   handle,
	}

	// Set finalizer to free the handle when GC'd
	runtime.SetFinalizer(req, func(r *TransactionRequest) {
		if r.handle != nil {
			C.pczt_transaction_request_free(r.handle)
		}
	})

	return req, nil
}

// Free explicitly frees the transaction request
func (r *TransactionRequest) Free() {
	if r.handle != nil {
		runtime.SetFinalizer(r, nil) // Clear finalizer to prevent double-free
		C.pczt_transaction_request_free(r.handle)
		r.handle = nil
	}
}

// TransparentInput represents a transparent UTXO to be spent
// This matches the binary serialization format expected by the Rust library
type TransparentInput struct {
	// Pubkey is the compressed secp256k1 public key (33 bytes)
	Pubkey []byte

	// TxID is the transaction ID of the UTXO being spent (32 bytes)
	TxID [32]byte

	// Vout is the output index in the previous transaction
	Vout uint32

	// Amount in zatoshis
	Amount uint64

	// ScriptPubKey is the script of the UTXO being spent
	ScriptPubKey []byte
}

// NewTransparentInput creates a new TransparentInput with validation.
//
// Parameters:
//   - pubkey: 33-byte compressed secp256k1 public key
//   - txid: 32-byte transaction ID
//   - vout: Output index in the previous transaction
//   - amount: Amount in zatoshis
//   - scriptPubKey: The scriptPubKey of the UTXO
//
// Returns an error if any parameter is invalid.
func NewTransparentInput(pubkey []byte, txid [32]byte, vout uint32, amount uint64, scriptPubKey []byte) (*TransparentInput, error) {
	if len(pubkey) != 33 {
		return nil, fmt.Errorf("invalid pubkey length: expected 33, got %d", len(pubkey))
	}
	if len(scriptPubKey) == 0 {
		return nil, errors.New("scriptPubKey must not be empty")
	}

	return &TransparentInput{
		Pubkey:       pubkey,
		TxID:         txid,
		Vout:         vout,
		Amount:       amount,
		ScriptPubKey: scriptPubKey,
	}, nil
}

// serializeTransparentInputs converts Go inputs to the binary format expected by Rust
func serializeTransparentInputs(inputs []TransparentInput) []byte {
	var buf []byte

	// Write number of inputs (u16 LE)
	numInputs := uint16(len(inputs))
	buf = append(buf, byte(numInputs), byte(numInputs>>8))

	for _, input := range inputs {
		// Write pubkey (33 bytes)
		buf = append(buf, input.Pubkey...)

		// Write txid (32 bytes)
		buf = append(buf, input.TxID[:]...)

		// Write vout (u32 LE)
		vout := input.Vout
		buf = append(buf, byte(vout), byte(vout>>8), byte(vout>>16), byte(vout>>24))

		// Write amount (u64 LE)
		amount := input.Amount
		buf = append(buf,
			byte(amount), byte(amount>>8), byte(amount>>16), byte(amount>>24),
			byte(amount>>32), byte(amount>>40), byte(amount>>48), byte(amount>>56),
		)

		// Write script length (u16 LE)
		scriptLen := uint16(len(input.ScriptPubKey))
		buf = append(buf, byte(scriptLen), byte(scriptLen>>8))

		// Write script
		buf = append(buf, input.ScriptPubKey...)
	}

	return buf
}

// PCZT represents a Partially Constructed Zcash Transaction
type PCZT struct {
	handle *C.PcztHandle
}

// newPCZT creates a new PCZT with automatic cleanup via finalizer
func newPCZT(handle *C.PcztHandle) *PCZT {
	p := &PCZT{handle: handle}
	runtime.SetFinalizer(p, func(pczt *PCZT) {
		if pczt.handle != nil {
			C.pczt_free(pczt.handle)
		}
	})
	return p
}

// Free explicitly frees the PCZT handle (optional - GC will handle automatically)
func (p *PCZT) Free() {
	if p.handle != nil {
		runtime.SetFinalizer(p, nil) // Clear finalizer to prevent double-free
		C.pczt_free(p.handle)
		p.handle = nil
	}
}

// consumeHandle returns the handle and clears it (transfers ownership)
func (p *PCZT) consumeHandle() *C.PcztHandle {
	if p.handle == nil {
		return nil
	}
	runtime.SetFinalizer(p, nil) // Clear finalizer - ownership transferred
	h := p.handle
	p.handle = nil
	return h
}

// ProposeTransaction creates a PCZT from transparent inputs and a transaction request.
//
// This implements the Creator, Constructor, and IO Finalizer roles.
//
// Parameters:
//   - inputs: List of transparent UTXOs to spend
//   - request: Transaction request with payment recipients
//
// Returns the created PCZT or an error.
func ProposeTransaction(inputs []TransparentInput, request *TransactionRequest) (*PCZT, error) {
	return ProposeTransactionWithChange(inputs, request, "")
}

// ProposeTransactionWithChange creates a PCZT with an explicit change address.
//
// This implements the Creator, Constructor, and IO Finalizer roles.
//
// Parameters:
//   - inputs: List of transparent UTXOs to spend
//   - request: Transaction request with payment recipients
//   - changeAddress: Optional transparent address for change. If empty, derives from first input's pubkey
//
// Returns the created PCZT or an error.
func ProposeTransactionWithChange(inputs []TransparentInput, request *TransactionRequest, changeAddress string) (*PCZT, error) {
	if len(inputs) == 0 {
		return nil, errors.New("at least one input is required")
	}
	if request == nil || request.handle == nil {
		return nil, errors.New("invalid transaction request")
	}

	// Serialize inputs to the binary format
	inputBytes := serializeTransparentInputs(inputs)

	// Convert change address to C string (nullable)
	var cChangeAddr *C.char
	if changeAddress != "" {
		cChangeAddr = C.CString(changeAddress)
		defer C.free(unsafe.Pointer(cChangeAddr))
	}

	var pcztHandle *C.PcztHandle
	code := C.pczt_propose_transaction(
		(*C.uint8_t)(unsafe.Pointer(&inputBytes[0])),
		C.size_t(len(inputBytes)),
		request.handle,
		cChangeAddr,
		&pcztHandle,
	)

	if code != C.SUCCESS {
		return nil, wrapError(ResultCode(code))
	}

	return newPCZT(pcztHandle), nil
}

// ProveTransaction adds Orchard proofs to a PCZT.
//
// This implements the Prover role. The proving key is embedded in the Rust library.
//
// This operation can be performed in parallel with signing operations.
//
// IMPORTANT: This function ALWAYS consumes the input PCZT, even on error.
// On error, the input PCZT is invalidated and cannot be reused.
// If you need to retry on failure, call SerializePCZT() before this function
// to create a backup that can be restored with ParsePCZT().
//
// Returns a new PCZT with proofs added.
func ProveTransaction(pczt *PCZT) (*PCZT, error) {
	if pczt == nil || pczt.handle == nil {
		return nil, errors.New("invalid PCZT")
	}

	// Consume input PCZT (transfers ownership to Rust)
	handle := pczt.consumeHandle()

	var outHandle *C.PcztHandle
	code := C.pczt_prove_transaction(handle, &outHandle)

	if code != C.SUCCESS {
		return nil, wrapError(ResultCode(code))
	}

	return newPCZT(outHandle), nil
}

// GetSighash gets the signature hash for a transparent input.
//
// This is the first step of the Signer role. After obtaining the sighash,
// you can sign it using your preferred signing infrastructure (including
// hardware wallets), then use AppendSignature to add the signature to the PCZT.
//
// Parameters:
//   - pczt: The PCZT to get the sighash from
//   - inputIndex: The index of the input to sign
//
// Returns the 32-byte signature hash.
func GetSighash(pczt *PCZT, inputIndex uint) ([32]byte, error) {
	if pczt == nil || pczt.handle == nil {
		return [32]byte{}, errors.New("invalid PCZT")
	}

	var sighash [32]byte
	code := C.pczt_get_sighash(
		pczt.handle,
		C.size_t(inputIndex),
		(*[32]C.uint8_t)(unsafe.Pointer(&sighash[0])),
	)

	if code != C.SUCCESS {
		return [32]byte{}, wrapError(ResultCode(code))
	}

	return sighash, nil
}

// AppendSignature adds a signature to the PCZT.
//
// This is the second step of the Signer role. The signature should be created
// by signing the sighash obtained from GetSighash.
//
// The implementation verifies that the signature is valid for the input being spent.
//
// IMPORTANT: This function ALWAYS consumes the input PCZT, even on error.
// On error, the input PCZT is invalidated and cannot be reused.
// If you need to retry on failure, call SerializePCZT() before this function
// to create a backup that can be restored with ParsePCZT().
//
// Parameters:
//   - pczt: The PCZT to add the signature to
//   - inputIndex: The index of the input being signed
//   - signature: The 64-byte ECDSA signature (r: 32 bytes, s: 32 bytes)
//
// Returns a new PCZT with the signature added.
func AppendSignature(pczt *PCZT, inputIndex uint, signature [64]byte) (*PCZT, error) {
	if pczt == nil || pczt.handle == nil {
		return nil, errors.New("invalid PCZT")
	}

	// Consume input PCZT (transfers ownership to Rust)
	handle := pczt.consumeHandle()

	var outHandle *C.PcztHandle
	code := C.pczt_append_signature(
		handle,
		C.size_t(inputIndex),
		(*[64]C.uint8_t)(unsafe.Pointer(&signature[0])),
		&outHandle,
	)

	if code != C.SUCCESS {
		return nil, wrapError(ResultCode(code))
	}

	return newPCZT(outHandle), nil
}

// FinalizeAndExtract finalizes the PCZT and extracts the transaction bytes.
//
// This implements the Spend Finalizer and Transaction Extractor roles.
//
// This performs final non-contextual verification and produces the raw
// transaction bytes ready to be broadcast to the Zcash network.
//
// IMPORTANT: This function ALWAYS consumes the input PCZT, even on error.
// On error, the input PCZT is invalidated and cannot be reused.
// If you need to retry on failure, call SerializePCZT() before this function
// to create a backup that can be restored with ParsePCZT().
//
// Returns the transaction bytes or an error.
func FinalizeAndExtract(pczt *PCZT) ([]byte, error) {
	if pczt == nil || pczt.handle == nil {
		return nil, errors.New("invalid PCZT")
	}

	// Consume input PCZT (transfers ownership to Rust)
	handle := pczt.consumeHandle()

	var txBytes *C.uint8_t
	var txBytesLen C.size_t

	code := C.pczt_finalize_and_extract(
		handle,
		&txBytes,
		&txBytesLen,
	)

	if code != C.SUCCESS {
		return nil, wrapError(ResultCode(code))
	}

	// Copy bytes to Go slice
	result := C.GoBytes(unsafe.Pointer(txBytes), C.int(txBytesLen))

	// Free the bytes allocated by Rust
	C.pczt_free_bytes(txBytes, txBytesLen)

	return result, nil
}

// ParsePCZT parses a PCZT from bytes.
//
// This is useful for receiving PCZTs that were serialized by another process
// or system (e.g., from a hardware wallet or remote signer).
//
// Returns the parsed PCZT or an error.
func ParsePCZT(pcztBytes []byte) (*PCZT, error) {
	if len(pcztBytes) == 0 {
		return nil, errors.New("empty PCZT bytes")
	}

	var handle *C.PcztHandle
	code := C.pczt_parse(
		(*C.uint8_t)(unsafe.Pointer(&pcztBytes[0])),
		C.size_t(len(pcztBytes)),
		&handle,
	)

	if code != C.SUCCESS {
		return nil, wrapError(ResultCode(code))
	}

	return newPCZT(handle), nil
}

// SerializePCZT serializes a PCZT to bytes.
//
// This is useful for transmitting PCZTs to another process or system
// (e.g., to a hardware wallet or remote signer).
//
// Returns the serialized bytes or an error.
func SerializePCZT(pczt *PCZT) ([]byte, error) {
	if pczt == nil || pczt.handle == nil {
		return nil, errors.New("invalid PCZT")
	}

	var bytes *C.uint8_t
	var bytesLen C.size_t

	code := C.pczt_serialize(
		pczt.handle,
		&bytes,
		&bytesLen,
	)

	if code != C.SUCCESS {
		return nil, wrapError(ResultCode(code))
	}

	// Copy bytes to Go slice
	result := C.GoBytes(unsafe.Pointer(bytes), C.int(bytesLen))

	// Free the bytes allocated by Rust
	C.pczt_free_bytes(bytes, bytesLen)

	return result, nil
}

// Combine merges multiple PCZTs into one.
//
// This is useful for parallel signing workflows where different parts of the
// transaction are processed independently and need to be merged.
//
// IMPORTANT: This function ALWAYS consumes ALL input PCZTs, even on error.
// On error, all input PCZTs are invalidated and cannot be reused.
// If you need to retry on failure, call SerializePCZT() on each PCZT before
// this function to create backups that can be restored with ParsePCZT().
//
// Parameters:
//   - pczts: Array of PCZTs to combine
//
// Returns the combined PCZT or an error.
func Combine(pczts []*PCZT) (*PCZT, error) {
	if len(pczts) == 0 {
		return nil, errors.New("at least one PCZT is required")
	}

	// Validate all PCZTs
	for i, pczt := range pczts {
		if pczt == nil || pczt.handle == nil {
			return nil, fmt.Errorf("invalid PCZT at index %d", i)
		}
	}

	// Consume all input PCZTs (transfers ownership to Rust)
	handles := make([]*C.PcztHandle, len(pczts))
	for i, pczt := range pczts {
		handles[i] = pczt.consumeHandle()
	}

	var outHandle *C.PcztHandle
	code := C.pczt_combine(
		&handles[0],
		C.uintptr_t(len(handles)),
		&outHandle,
	)

	if code != C.SUCCESS {
		return nil, wrapError(ResultCode(code))
	}

	return newPCZT(outHandle), nil
}

// TransparentOutput represents a transparent transaction output.
// This is used for verifying expected change outputs.
type TransparentOutput struct {
	// ScriptPubKey is the P2PKH script of the output (raw bytes, no CompactSize prefix)
	ScriptPubKey []byte

	// Value in zatoshis
	Value uint64
}

// VerifyBeforeSigning verifies the PCZT before signing.
//
// This function performs verification checks to ensure the PCZT matches
// the expected transaction request and change outputs.
//
// Parameters:
//   - pczt: The PCZT to verify (not consumed)
//   - request: The original transaction request
//   - expectedChange: Expected change outputs for verification
//
// Returns an error if verification fails.
func VerifyBeforeSigning(pczt *PCZT, request *TransactionRequest, expectedChange []TransparentOutput) error {
	if pczt == nil || pczt.handle == nil {
		return errors.New("invalid PCZT")
	}
	if request == nil || request.handle == nil {
		return errors.New("invalid transaction request")
	}

	// Convert expectedChange to C array, copying script data to C memory
	// to avoid CGO pointer rules violation
	cOutputs := make([]C.CTransparentOutput, len(expectedChange))
	scriptPtrs := make([]unsafe.Pointer, len(expectedChange)) // Track for cleanup

	for i, output := range expectedChange {
		cOutputs[i].value = C.uint64_t(output.Value)
		if len(output.ScriptPubKey) > 0 {
			// Copy to C memory to avoid "Go pointer to Go pointer" issue
			scriptPtrs[i] = C.CBytes(output.ScriptPubKey)
			cOutputs[i].script_pub_key = (*C.uchar)(scriptPtrs[i])
			cOutputs[i].script_pub_key_len = C.uintptr_t(len(output.ScriptPubKey))
		}
	}

	// Ensure cleanup of C-allocated memory
	defer func() {
		for _, ptr := range scriptPtrs {
			if ptr != nil {
				C.free(ptr)
			}
		}
	}()

	var cOutputsPtr *C.CTransparentOutput
	if len(cOutputs) > 0 {
		cOutputsPtr = &cOutputs[0]
	}

	code := C.pczt_verify_before_signing(
		pczt.handle,
		request.handle,
		cOutputsPtr,
		C.uintptr_t(len(expectedChange)),
	)

	if code != C.SUCCESS {
		return wrapError(ResultCode(code))
	}

	return nil
}

// SetTargetHeight sets the target block height for consensus branch ID selection.
//
// This is important for ensuring the transaction uses the correct consensus rules.
//
// Parameters:
//   - height: The target block height
func (r *TransactionRequest) SetTargetHeight(height uint32) error {
	if r == nil || r.handle == nil {
		return errors.New("invalid transaction request")
	}

	code := C.pczt_transaction_request_set_target_height(
		r.handle,
		C.uint32_t(height),
	)

	if code != C.SUCCESS {
		return wrapError(ResultCode(code))
	}

	return nil
}

// SetUseMainnet sets whether to use mainnet parameters for consensus branch ID.
//
// By default, the library uses mainnet parameters. Set this to false for testnet.
// Regtest networks (like Zebra's regtest) typically use mainnet-like branch IDs,
// so keep the default (true) for regtest.
//
// Parameters:
//   - useMainnet: True for mainnet/regtest, false for testnet
func (r *TransactionRequest) SetUseMainnet(useMainnet bool) error {
	if r == nil || r.handle == nil {
		return errors.New("invalid transaction request")
	}

	code := C.pczt_transaction_request_set_use_mainnet(
		r.handle,
		C.bool(useMainnet),
	)

	if code != C.SUCCESS {
		return wrapError(ResultCode(code))
	}

	return nil
}

// NewTransactionRequestWithTargetHeight creates a new transaction request
// with a specific target block height.
//
// This is a convenience function that combines NewTransactionRequest and SetTargetHeight.
func NewTransactionRequestWithTargetHeight(payments []Payment, targetHeight uint32) (*TransactionRequest, error) {
	req, err := NewTransactionRequest(payments)
	if err != nil {
		return nil, err
	}

	err = req.SetTargetHeight(targetHeight)
	if err != nil {
		req.Free()
		return nil, err
	}

	return req, nil
}

// CalculateFee calculates the ZIP-317 transaction fee.
//
// This is a pure function that computes the fee based on transaction shape.
// Use this to calculate fees before building a transaction, e.g., for "send max"
// functionality where you need to know the fee to calculate the maximum sendable amount.
//
// Parameters:
//   - numTransparentInputs: Number of transparent UTXOs to spend
//   - numTransparentOutputs: Number of transparent outputs (including change if any)
//   - numOrchardOutputs: Number of Orchard (shielded) outputs
//
// Returns the fee in zatoshis.
//
// Example:
//
//	// Transparent-only: 1 input, 2 outputs (1 payment + 1 change)
//	fee := CalculateFee(1, 2, 0) // Returns 10000
//
//	// Shielded: 1 input, 1 change, 1 orchard output
//	fee := CalculateFee(1, 1, 1) // Returns 15000
//
//	// Calculate max sendable amount
//	totalInput := uint64(100000000) // 1 ZEC
//	fee := CalculateFee(1, 2, 0)    // 10000 zatoshis
//	maxSend := totalInput - fee     // 99990000 zatoshis
//
// See ZIP-317: https://zips.z.cash/zip-0317
func CalculateFee(numTransparentInputs, numTransparentOutputs, numOrchardOutputs int) uint64 {
	return uint64(C.pczt_calculate_fee(
		C.uintptr_t(numTransparentInputs),
		C.uintptr_t(numTransparentOutputs),
		C.uintptr_t(numOrchardOutputs),
	))
}
