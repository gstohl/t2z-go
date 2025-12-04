// Device B - Offline Signer (Hardware Wallet Simulation)
// Signs sighash and returns signature
package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

func main() {
	env := loadEnv()
	address := env["ADDRESS"]

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  DEVICE B - OFFLINE SIGNER (Hardware Wallet Simulation)")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("\nThis device holds the private key and signs transactions.")
	fmt.Println("In production, this would be an air-gapped hardware wallet.")
	fmt.Printf("\nWallet address: %s\n\n", address)

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Paste the sighash from Device A:\n")
	fmt.Print("SIGHASH: ")
	sighashHex, _ := reader.ReadString('\n')
	sighashHex = strings.TrimSpace(sighashHex)

	if len(sighashHex) != 64 {
		fmt.Println("\nInvalid sighash (expected 32 bytes / 64 hex chars). Exiting.")
		os.Exit(1)
	}

	sighash, _ := hex.DecodeString(sighashHex)

	fmt.Println("\nSigning...")

	privKeyBytes, _ := hex.DecodeString(env["PRIVATE_KEY"])
	privKey := secp256k1.PrivKeyFromBytes(privKeyBytes)

	sig := ecdsa.SignCompact(privKey, sighash, true)
	// Extract 64-byte signature (skip recovery byte)
	sigHex := hex.EncodeToString(sig[1:65])

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  SIGNATURE READY")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("\nCopy this signature back to Device A:\n")
	fmt.Printf("SIGNATURE: %s\n", sigHex)
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("\nThe private key stayed on this device!")
}

func loadEnv() map[string]string {
	envPath := filepath.Join(getDir(), "..", "..", ".env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		fmt.Println("No .env file found. Run: go run ./cmd/generate-wallet")
		os.Exit(1)
	}
	env := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		if idx := strings.Index(line, "="); idx > 0 && !strings.HasPrefix(line, "#") {
			key := strings.TrimSpace(line[:idx])
			val := strings.Trim(strings.TrimSpace(line[idx+1:]), "\"'")
			env[key] = val
		}
	}
	return env
}

func getDir() string {
	exe, _ := os.Executable()
	return filepath.Dir(exe)
}
