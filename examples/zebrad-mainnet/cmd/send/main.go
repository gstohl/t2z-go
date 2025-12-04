// Interactive Send - Reads wallet from .env, prompts for recipients, sends transaction
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/ripemd160"
	"t2z"
)

type Recipient struct {
	Address string
	Amount  uint64
	Memo    string
}

func main() {
	env := loadEnv()
	zebraRPC := fmt.Sprintf("http://%s:%s", env["ZEBRA_HOST"], env["ZEBRA_PORT"])

	privKeyBytes, _ := hex.DecodeString(env["PRIVATE_KEY"])
	privKey := secp256k1.PrivKeyFromBytes(privKeyBytes)
	pubkey := privKey.PubKey().SerializeCompressed()
	address := env["ADDRESS"]

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  t2z Mainnet Send")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("\nYour address: %s\n\n", address)

	// Fetch UTXOs
	fmt.Print("Fetching balance... ")
	utxos, err := getUTXOs(zebraRPC, address)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("done")

	var totalSats int64
	for _, u := range utxos {
		totalSats += u.Satoshis
	}

	if len(utxos) == 0 {
		fmt.Println("\nNo UTXOs found. Send ZEC to this address first.")
		return
	}

	fmt.Printf("\nBalance: %.8f ZEC (%d UTXO%s)\n\n", float64(totalSats)/1e8, len(utxos), plural(len(utxos)))

	// Interactive recipient input
	reader := bufio.NewReader(os.Stdin)
	var recipients []Recipient

	fmt.Println("Enter recipients (shielded addresses starting with 'u' recommended)")
	fmt.Println("Press Enter with empty address to finish.\n")

	for {
		fmt.Printf("Recipient %d address: ", len(recipients)+1)
		addr, _ := reader.ReadString('\n')
		addr = strings.TrimSpace(addr)
		if addr == "" {
			break
		}

		fmt.Print("Amount in ZEC: ")
		amountStr, _ := reader.ReadString('\n')
		amountZec, err := strconv.ParseFloat(strings.TrimSpace(amountStr), 64)
		if err != nil || amountZec <= 0 {
			fmt.Println("Invalid amount, skipping.\n")
			continue
		}

		amountSats := uint64(amountZec * 1e8)

		// Ask for memo if shielded
		var memo string
		if !strings.HasPrefix(addr, "t") {
			fmt.Print("Memo (optional, press Enter to skip): ")
			memo, _ = reader.ReadString('\n')
			memo = strings.TrimSpace(memo)
		}

		recipients = append(recipients, Recipient{Address: addr, Amount: amountSats, Memo: memo})
		memoInfo := ""
		if memo != "" {
			memoInfo = fmt.Sprintf(" [memo: \"%s\"]", truncate(memo, 20))
		}
		fmt.Printf("Added: %.8f ZEC → %s...%s\n\n", amountZec, truncate(addr, 30), memoInfo)
	}

	if len(recipients) == 0 {
		fmt.Println("\nNo recipients entered. Exiting.")
		return
	}

	// Calculate fee
	numTransparent := 0
	numShielded := 0
	for _, r := range recipients {
		if strings.HasPrefix(r.Address, "t") {
			numTransparent++
		} else {
			numShielded++
		}
	}
	fee := t2z.CalculateFee(len(utxos), numTransparent+1, numShielded)

	var totalSend uint64
	for _, r := range recipients {
		totalSend += r.Amount
	}
	totalNeeded := totalSend + fee

	fmt.Println("\n--- Transaction Summary ---")
	for _, r := range recipients {
		memoInfo := ""
		if r.Memo != "" {
			memoInfo = " [memo]"
		}
		fmt.Printf("  %.8f ZEC → %s...%s\n", float64(r.Amount)/1e8, truncate(r.Address, 40), memoInfo)
	}
	fmt.Printf("  Fee: %.8f ZEC\n", float64(fee)/1e8)
	fmt.Printf("  Total: %.8f ZEC\n", float64(totalNeeded)/1e8)

	if totalNeeded > uint64(totalSats) {
		fmt.Printf("\nInsufficient balance! Need %.8f ZEC\n", float64(totalNeeded)/1e8)
		os.Exit(1)
	}

	// Build inputs
	h := sha256.Sum256(pubkey)
	r := ripemd160.New()
	r.Write(h[:])
	pkh := r.Sum(nil)
	script := append([]byte{0x76, 0xa9, 0x14}, pkh...)
	script = append(script, 0x88, 0xac)

	var inputs []t2z.TransparentInput
	var inputTotal uint64
	for _, utxo := range utxos {
		txid, _ := hex.DecodeString(utxo.Txid)
		// Reverse txid bytes
		for i, j := 0, len(txid)-1; i < j; i, j = i+1, j-1 {
			txid[i], txid[j] = txid[j], txid[i]
		}
		var txidArr [32]byte
		copy(txidArr[:], txid)

		inputs = append(inputs, t2z.TransparentInput{
			Pubkey:       pubkey,
			TxID:         txidArr,
			Vout:         uint32(utxo.OutputIndex),
			Amount:       uint64(utxo.Satoshis),
			ScriptPubKey: script,
		})
		inputTotal += uint64(utxo.Satoshis)
		if inputTotal >= totalNeeded {
			break
		}
	}

	// Build payments
	var payments []t2z.Payment
	for _, rec := range recipients {
		payments = append(payments, t2z.Payment{
			Address: rec.Address,
			Amount:  rec.Amount,
			Memo:    rec.Memo,
		})
	}

	// Get block height
	blockHeight, _ := getBlockHeight(zebraRPC)

	// Build transaction
	fmt.Println("\nBuilding transaction...")

	fmt.Print("  Proposing... ")
	request, _ := t2z.NewTransactionRequest(payments)
	defer request.Free()
	request.SetTargetHeight(uint32(blockHeight + 10))

	pczt, err := t2z.ProposeTransaction(inputs, request)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("done")

	fmt.Print("  Proving... ")
	proved, err := t2z.ProveTransaction(pczt)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("done")

	fmt.Print("  Signing... ")
	signed := proved
	for i := range inputs {
		sighash, _ := t2z.GetSighash(signed, uint(i))
		sig := ecdsa.SignCompact(privKey, sighash[:], true)
		var sigBytes [64]byte
		copy(sigBytes[:], sig[1:])
		signed, _ = t2z.AppendSignature(signed, uint(i), sigBytes)
	}
	fmt.Println("done")

	fmt.Print("  Finalizing... ")
	txBytes, _ := t2z.FinalizeAndExtract(signed)
	fmt.Println("done")

	fmt.Print("  Broadcasting... ")
	txid, err := broadcast(zebraRPC, hex.EncodeToString(txBytes))
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("done")

	fmt.Println("\nTransaction sent!")
	fmt.Printf("TXID: %s\n", txid)
}

type UTXO struct {
	Txid        string `json:"txid"`
	OutputIndex int    `json:"outputIndex"`
	Satoshis    int64  `json:"satoshis"`
}

func getUTXOs(rpcURL, address string) ([]UTXO, error) {
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "getaddressutxos",
		"params":  []any{map[string]any{"addresses": []string{address}}},
		"id":      1,
	})
	resp, err := http.Post(rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		Result []UTXO `json:"result"`
		Error  *struct{ Message string } `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Error != nil {
		return nil, fmt.Errorf("%s", result.Error.Message)
	}
	return result.Result, nil
}

func getBlockHeight(rpcURL string) (int, error) {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "getblockchaininfo", "params": []any{}, "id": 1})
	resp, err := http.Post(rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var result struct {
		Result struct{ Blocks int } `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Result.Blocks, nil
}

func broadcast(rpcURL, txHex string) (string, error) {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "sendrawtransaction", "params": []string{txHex}, "id": 1})
	resp, err := http.Post(rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		Result string `json:"result"`
		Error  *struct{ Message string } `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Error != nil {
		return "", fmt.Errorf("%s", result.Error.Message)
	}
	return result.Result, nil
}

func loadEnv() map[string]string {
	envPath := ".env"
	data, err := os.ReadFile(envPath)
	if err != nil {
		fmt.Println("No .env file found. Run: go run ./cmd/generate-wallet")
		os.Exit(1)
	}
	env := make(map[string]string)
	env["ZEBRA_HOST"] = "localhost"
	env["ZEBRA_PORT"] = "8232"
	for _, line := range strings.Split(string(data), "\n") {
		if idx := strings.Index(line, "="); idx > 0 && !strings.HasPrefix(line, "#") {
			key := strings.TrimSpace(line[:idx])
			val := strings.Trim(strings.TrimSpace(line[idx+1:]), "\"'")
			env[key] = val
		}
	}
	return env
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
