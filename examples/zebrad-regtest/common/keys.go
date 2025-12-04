package common

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/ripemd160"
)

// ZcashKeypair represents a Zcash transparent keypair
type ZcashKeypair struct {
	PrivateKey []byte
	PublicKey  []byte
	Address    string
	WIF        string
}

// Zcash testnet/regtest P2PKH version bytes
var zcashTestnetP2PKH = []byte{0x1d, 0x25}

// TEST_KEYPAIR is the pre-generated test keypair matching TypeScript
// Private key: e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35
// Address: tmEUfekwCArJoFTMEL2kFwQyrsDMCNX5ZFf
var TEST_KEYPAIR *ZcashKeypair

func init() {
	privateKeyHex := "e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35"
	privateKeyBytes, _ := hex.DecodeString(privateKeyHex)
	TEST_KEYPAIR = KeypairFromPrivateKey(privateKeyBytes)
}

// KeypairFromPrivateKey creates a keypair from a private key
func KeypairFromPrivateKey(privateKey []byte) *ZcashKeypair {
	privKey := secp256k1.PrivKeyFromBytes(privateKey)
	pubKey := privKey.PubKey().SerializeCompressed()

	address := PubkeyToAddress(pubKey)
	wif := PrivateKeyToWIF(privateKey)

	return &ZcashKeypair{
		PrivateKey: privateKey,
		PublicKey:  pubKey,
		Address:    address,
		WIF:        wif,
	}
}

// Hash160 computes RIPEMD160(SHA256(data))
func Hash160(data []byte) []byte {
	sha256Hash := sha256.Sum256(data)
	ripemd160Hasher := ripemd160.New()
	ripemd160Hasher.Write(sha256Hash[:])
	return ripemd160Hasher.Sum(nil)
}

// DoubleSHA256 computes SHA256(SHA256(data))
func DoubleSHA256(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:]
}

// PubkeyToAddress converts a public key to a Zcash testnet address
func PubkeyToAddress(pubkey []byte) string {
	hash := Hash160(pubkey)

	// Zcash uses 2-byte version prefix
	payload := append(zcashTestnetP2PKH, hash...)

	// Base58check encode
	return Base58CheckEncode(payload)
}

// PrivateKeyToWIF converts a private key to WIF format
func PrivateKeyToWIF(privateKey []byte) string {
	// Testnet WIF version byte
	version := byte(0xef)

	// Add version byte and compression flag
	payload := make([]byte, 0, 34)
	payload = append(payload, version)
	payload = append(payload, privateKey...)
	payload = append(payload, 0x01) // compressed

	return Base58CheckEncode(payload)
}

// CreateP2PKHScript creates a P2PKH script for the given public key
func CreateP2PKHScript(pubkey []byte) []byte {
	pubkeyHash := Hash160(pubkey)
	script := make([]byte, 25)
	script[0] = 0x76 // OP_DUP
	script[1] = 0xa9 // OP_HASH160
	script[2] = 0x14 // PUSH 20 bytes
	copy(script[3:23], pubkeyHash)
	script[23] = 0x88 // OP_EQUALVERIFY
	script[24] = 0xac // OP_CHECKSIG
	return script
}

// SignCompact signs a message hash and returns a 64-byte compact signature
func SignCompact(messageHash []byte, keypair *ZcashKeypair) [64]byte {
	privKey := secp256k1.PrivKeyFromBytes(keypair.PrivateKey)

	var hash [32]byte
	copy(hash[:], messageHash)

	compact := ecdsa.SignCompact(privKey, hash[:], true)

	var sigBytes [64]byte
	copy(sigBytes[:], compact[1:]) // Skip recovery ID
	return sigBytes
}

// Base58 alphabet
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// Base58CheckEncode encodes data with a checksum
func Base58CheckEncode(payload []byte) string {
	checksum := DoubleSHA256(payload)[:4]
	data := append(payload, checksum...)
	return Base58Encode(data)
}

// Base58Encode encodes bytes to base58
func Base58Encode(input []byte) string {
	if len(input) == 0 {
		return ""
	}

	// Count leading zeros
	leadingZeros := 0
	for _, b := range input {
		if b == 0 {
			leadingZeros++
		} else {
			break
		}
	}

	// Convert to big integer and encode
	size := len(input)*138/100 + 1
	output := make([]byte, size)

	for _, b := range input {
		carry := int(b)
		for i := size - 1; i >= 0; i-- {
			carry += 256 * int(output[i])
			output[i] = byte(carry % 58)
			carry /= 58
		}
	}

	// Skip leading zeros in output
	startIdx := 0
	for startIdx < len(output) && output[startIdx] == 0 {
		startIdx++
	}

	// Build result
	result := make([]byte, leadingZeros+len(output)-startIdx)
	for i := 0; i < leadingZeros; i++ {
		result[i] = '1'
	}
	for i := startIdx; i < len(output); i++ {
		result[leadingZeros+i-startIdx] = base58Alphabet[output[i]]
	}

	return string(result)
}

// BytesToHex converts bytes to hex string
func BytesToHex(data []byte) string {
	return hex.EncodeToString(data)
}

// HexToBytes converts hex string to bytes
func HexToBytes(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

// ReverseBytes reverses a byte slice (for txid endianness)
func ReverseBytes(data []byte) []byte {
	result := make([]byte, len(data))
	for i := 0; i < len(data); i++ {
		result[i] = data[len(data)-1-i]
	}
	return result
}
