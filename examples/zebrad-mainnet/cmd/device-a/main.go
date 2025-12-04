// Device A - Online Device (Hardware Wallet Simulation)
// Builds transaction, outputs sighash, waits for signature, broadcasts
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

	"golang.org/x/crypto/ripemd160"
	"t2z"
)

func main() {
	env := loadEnv()
	zebraRPC := fmt.Sprintf("http://%s:%s", env["ZEBRA_HOST"], env["ZEBRA_PORT"])

	pubkey, _ := hex.DecodeString(env["PUBLIC_KEY"])
	address := env["ADDRESS"]

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  DEVICE A - ONLINE DEVICE (Hardware Wallet Simulation)")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("\nThis device builds transactions but NEVER sees the private key!")
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

	fmt.Printf("\nBalance: %.8f ZEC\n\n", float64(totalSats)/1e8)

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Recipient address: ")
	recipientAddr, _ := reader.ReadString('\n')
	recipientAddr = strings.TrimSpace(recipientAddr)
	if recipientAddr == "" {
		fmt.Println("No address entered. Exiting.")
		return
	}

	fmt.Print("Amount in ZEC: ")
	amountStr, _ := reader.ReadString('\n')
	amountZec, err := strconv.ParseFloat(strings.TrimSpace(amountStr), 64)
	if err != nil || amountZec <= 0 {
		fmt.Println("Invalid amount. Exiting.")
		os.Exit(1)
	}
	amountSats := uint64(amountZec * 1e8)

	// Optional memo
	var memo string
	if !strings.HasPrefix(recipientAddr, "t") {
		fmt.Print("Memo (optional, press Enter to skip): ")
		memo, _ = reader.ReadString('\n')
		memo = strings.TrimSpace(memo)
	}

	// Calculate fee
	isShielded := !strings.HasPrefix(recipientAddr, "t")
	var fee uint64
	if isShielded {
		fee = t2z.CalculateFee(1, 1, 1)
	} else {
		fee = t2z.CalculateFee(1, 2, 0)
	}
	totalNeeded := amountSats + fee

	if totalNeeded > uint64(totalSats) {
		fmt.Printf("\nInsufficient balance! Need %.8f ZEC\n", float64(totalNeeded)/1e8)
		os.Exit(1)
	}

	fmt.Println("\n--- Transaction Summary ---")
	fmt.Printf("  To: %s\n", recipientAddr)
	fmt.Printf("  Amount: %.8f ZEC\n", amountZec)
	if memo != "" {
		fmt.Printf("  Memo: \"%s\"\n", memo)
	}
	fmt.Printf("  Fee: %.8f ZEC\n", float64(fee)/1e8)

	// Build input
	h := sha256.Sum256(pubkey)
	r := ripemd160.New()
	r.Write(h[:])
	pkh := r.Sum(nil)
	script := append([]byte{0x76, 0xa9, 0x14}, pkh...)
	script = append(script, 0x88, 0xac)

	utxo := utxos[0]
	txid, _ := hex.DecodeString(utxo.Txid)
	// Reverse txid
	for i, j := 0, len(txid)-1; i < j; i, j = i+1, j-1 {
		txid[i], txid[j] = txid[j], txid[i]
	}
	var txidArr [32]byte
	copy(txidArr[:], txid)

	input := t2z.TransparentInput{
		Pubkey:       pubkey,
		TxID:         txidArr,
		Vout:         uint32(utxo.OutputIndex),
		Amount:       uint64(utxo.Satoshis),
		ScriptPubKey: script,
	}

	payment := t2z.Payment{Address: recipientAddr, Amount: amountSats, Memo: memo}

	blockHeight, _ := getBlockHeight(zebraRPC)

	// Build and prove transaction
	fmt.Println("\nBuilding transaction...")

	fmt.Print("  Proposing... ")
	request, _ := t2z.NewTransactionRequest([]t2z.Payment{payment})
	defer request.Free()
	request.SetTargetHeight(uint32(blockHeight + 10))

	pczt, err := t2z.ProposeTransaction([]t2z.TransparentInput{input}, request)
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

	// Get sighash
	sighash, _ := t2z.GetSighash(proved, 0)
	sighashHex := hex.EncodeToString(sighash[:])

	// Serialize PCZT
	psztBytes, _ := t2z.SerializePCZT(proved)
	psztHex := hex.EncodeToString(psztBytes)

	// Save to temp file
	tempFile := ".pczt-temp"
	os.WriteFile(tempFile, []byte(psztHex), 0600)

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  SIGHASH READY FOR OFFLINE SIGNING")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("\nCopy this sighash to Device B:\n")
	fmt.Printf("SIGHASH: %s\n", sighashHex)
	fmt.Println("\n" + strings.Repeat("=", 60))

	// Wait for signature
	fmt.Println("\nRun Device B with the sighash, then paste the signature here.\n")
	fmt.Print("Paste signature from Device B: ")
	sigHex, _ := reader.ReadString('\n')
	sigHex = strings.TrimSpace(sigHex)

	if len(sigHex) != 128 {
		fmt.Println("\nInvalid signature (expected 64 bytes / 128 hex chars). Exiting.")
		os.Exit(1)
	}

	sigBytes, _ := hex.DecodeString(sigHex)
	var sig [64]byte
	copy(sig[:], sigBytes)

	// Load PCZT and finalize
	fmt.Println("\nFinalizing transaction...")
	psztData, _ := os.ReadFile(tempFile)
	loadedPczt, _ := t2z.ParsePCZT(mustHex(string(psztData)))
	signed, _ := t2z.AppendSignature(loadedPczt, 0, sig)

	fmt.Print("  Extracting... ")
	txBytes, _ := t2z.FinalizeAndExtract(signed)
	fmt.Println("done")

	fmt.Print("  Broadcasting... ")
	txidResult, err := broadcast(zebraRPC, hex.EncodeToString(txBytes))
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("done")

	// Cleanup
	os.Remove(tempFile)

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  TRANSACTION BROADCAST SUCCESSFUL!")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("\nTXID: %s\n", txidResult)
	fmt.Println("\nThe private key NEVER touched this device!")
}

type UTXO struct {
	Txid        string `json:"txid"`
	OutputIndex int    `json:"outputIndex"`
	Satoshis    int64  `json:"satoshis"`
}

func getUTXOs(rpcURL, address string) ([]UTXO, error) {
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "method": "getaddressutxos",
		"params": []any{map[string]any{"addresses": []string{address}}}, "id": 1,
	})
	resp, _ := http.Post(rpcURL, "application/json", bytes.NewReader(body))
	defer resp.Body.Close()
	var result struct {
		Result []UTXO                   `json:"result"`
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
	resp, _ := http.Post(rpcURL, "application/json", bytes.NewReader(body))
	defer resp.Body.Close()
	var result struct{ Result struct{ Blocks int } }
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Result.Blocks, nil
}

func broadcast(rpcURL, txHex string) (string, error) {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "sendrawtransaction", "params": []string{txHex}, "id": 1})
	resp, _ := http.Post(rpcURL, "application/json", bytes.NewReader(body))
	defer resp.Body.Close()
	var result struct {
		Result string                   `json:"result"`
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
	data, _ := os.ReadFile(envPath)
	env := map[string]string{"ZEBRA_HOST": "localhost", "ZEBRA_PORT": "8232"}
	for _, line := range strings.Split(string(data), "\n") {
		if idx := strings.Index(line, "="); idx > 0 && !strings.HasPrefix(line, "#") {
			env[strings.TrimSpace(line[:idx])] = strings.Trim(strings.TrimSpace(line[idx+1:]), "\"'")
		}
	}
	return env
}

func mustHex(s string) []byte {
	b, _ := hex.DecodeString(s)
	return b
}
