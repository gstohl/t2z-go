// Generate Wallet - Creates a new wallet and saves to .env file
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"golang.org/x/crypto/ripemd160"
)

func main() {
	envPath := filepath.Join(getDir(), "..", ".env")

	// Check if wallet already exists
	if _, err := os.Stat(envPath); err == nil {
		fmt.Println("Wallet already exists at .env")
		fmt.Println("Delete .env first if you want to generate a new wallet.")
		if env, err := os.ReadFile(envPath); err == nil {
			for _, line := range splitLines(string(env)) {
				if len(line) > 8 && line[:8] == "ADDRESS=" {
					fmt.Printf("\nCurrent address: %s\n", line[8:])
				}
			}
		}
		return
	}

	// Generate random private key
	privKeyBytes := make([]byte, 32)
	if _, err := rand.Read(privKeyBytes); err != nil {
		fmt.Printf("Error generating random bytes: %v\n", err)
		os.Exit(1)
	}

	privKey := secp256k1.PrivKeyFromBytes(privKeyBytes)
	pubkey := privKey.PubKey().SerializeCompressed()
	address := pubkeyToMainnetAddress(pubkey)

	// Build .env content
	envContent := fmt.Sprintf(`# Zcash Mainnet Wallet
# Generated: %s
# WARNING: Keep this file secret! Never commit to git.

PRIVATE_KEY=%s
PUBLIC_KEY=%s
ADDRESS=%s

# Zebra RPC (mainnet default port)
ZEBRA_HOST=localhost
ZEBRA_PORT=8232
`, time.Now().Format(time.RFC3339),
		hex.EncodeToString(privKeyBytes),
		hex.EncodeToString(pubkey),
		address)

	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		fmt.Printf("Error writing .env: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("New wallet generated!\n")
	fmt.Printf("Address: %s\n", address)
	fmt.Printf("\nSaved to: %s\n", envPath)
	fmt.Println("\nIMPORTANT: Back up your private key securely!")
}

func pubkeyToMainnetAddress(pubkey []byte) string {
	h := sha256.Sum256(pubkey)
	r := ripemd160.New()
	r.Write(h[:])
	pkh := r.Sum(nil)
	data := append([]byte{0x1c, 0xb8}, pkh...) // mainnet prefix
	check := sha256.Sum256(data)
	check = sha256.Sum256(check[:])
	return base58Encode(append(data, check[:4]...))
}

func base58Encode(data []byte) string {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	var result []byte
	for _, b := range data {
		carry := int(b)
		for i := len(result) - 1; i >= 0; i-- {
			carry += 256 * int(result[i])
			result[i] = byte(carry % 58)
			carry /= 58
		}
		for carry > 0 {
			result = append([]byte{byte(carry % 58)}, result...)
			carry /= 58
		}
	}
	for _, b := range data {
		if b != 0 {
			break
		}
		result = append([]byte{0}, result...)
	}
	out := make([]byte, len(result))
	for i, b := range result {
		out[i] = alphabet[b]
	}
	return string(out)
}

func getDir() string {
	exe, _ := os.Executable()
	return filepath.Dir(exe)
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
