package common

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	t2z "github.com/gstohl/t2z/go"
)

// Data directory for storing spent UTXOs and test data
var dataDir string

func init() {
	// Get executable directory for data storage
	exe, err := os.Executable()
	if err != nil {
		dataDir = "data"
	} else {
		dataDir = filepath.Join(filepath.Dir(exe), "..", "data")
	}
	// Also try current working directory
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		dataDir = "data"
	}
}

// SetDataDir sets the data directory
func SetDataDir(dir string) {
	dataDir = dir
}

// LoadSpentUtxos loads the set of spent UTXOs from file
func LoadSpentUtxos() map[string]bool {
	spent := make(map[string]bool)

	filePath := filepath.Join(dataDir, "spent-utxos.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return spent
	}

	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return spent
	}

	for _, key := range arr {
		spent[key] = true
	}
	return spent
}

// SaveSpentUtxos saves the set of spent UTXOs to file
func SaveSpentUtxos(spent map[string]bool) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	arr := make([]string, 0, len(spent))
	for key := range spent {
		arr = append(arr, key)
	}

	data, err := json.MarshalIndent(arr, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dataDir, "spent-utxos.json"), data, 0644)
}

// MarkUtxosSpent marks UTXOs as spent
func MarkUtxosSpent(inputs []t2z.TransparentInput) error {
	spent := LoadSpentUtxos()
	for _, input := range inputs {
		key := fmt.Sprintf("%s:%d", BytesToHex(input.TxID[:]), input.Vout)
		spent[key] = true
	}
	return SaveSpentUtxos(spent)
}

// ClearSpentUtxos clears the spent UTXO tracker
func ClearSpentUtxos() error {
	return SaveSpentUtxos(make(map[string]bool))
}

// ZatoshiToZec converts zatoshis to ZEC string
func ZatoshiToZec(zatoshi uint64) string {
	return fmt.Sprintf("%.8f", float64(zatoshi)/100_000_000)
}

// TxOutput represents a parsed transaction output
type TxOutput struct {
	Value        uint64
	ScriptPubKey []byte
}

// ParseTxOutputs parses transaction outputs from raw tx hex
func ParseTxOutputs(txHex string) ([]TxOutput, error) {
	tx, err := HexToBytes(txHex)
	if err != nil {
		return nil, err
	}

	offset := 0

	// Skip header (4 bytes version + 4 bytes version group id)
	offset += 8

	// Read vin count (varint - simplified, assuming single byte)
	if offset >= len(tx) {
		return nil, fmt.Errorf("tx too short for vin count")
	}
	vinCount := int(tx[offset])
	offset++

	// Skip all inputs
	for i := 0; i < vinCount; i++ {
		offset += 32 // prev txid
		offset += 4  // prev vout
		if offset >= len(tx) {
			return nil, fmt.Errorf("tx too short for script length")
		}
		scriptLen := int(tx[offset])
		offset += 1 + scriptLen // script length + script
		offset += 4             // sequence
	}

	// Read vout count
	if offset >= len(tx) {
		return nil, fmt.Errorf("tx too short for vout count")
	}
	voutCount := int(tx[offset])
	offset++

	outputs := make([]TxOutput, 0, voutCount)

	for i := 0; i < voutCount; i++ {
		if offset+8 > len(tx) {
			return nil, fmt.Errorf("tx too short for value")
		}
		value := binary.LittleEndian.Uint64(tx[offset : offset+8])
		offset += 8

		if offset >= len(tx) {
			return nil, fmt.Errorf("tx too short for script pubkey length")
		}
		scriptLen := int(tx[offset])
		offset++

		if offset+scriptLen > len(tx) {
			return nil, fmt.Errorf("tx too short for script pubkey")
		}
		scriptPubKey := make([]byte, scriptLen)
		copy(scriptPubKey, tx[offset:offset+scriptLen])
		offset += scriptLen

		outputs = append(outputs, TxOutput{
			Value:        value,
			ScriptPubKey: scriptPubKey,
		})
	}

	return outputs, nil
}

// ComputeTxid computes the txid from raw transaction hex
func ComputeTxid(txHex string) (string, error) {
	tx, err := HexToBytes(txHex)
	if err != nil {
		return "", err
	}
	hash := DoubleSHA256(tx)
	return BytesToHex(ReverseBytes(hash)), nil
}

// GetCoinbaseUtxo gets a coinbase UTXO from a block
func GetCoinbaseUtxo(client *ZebraClient, blockHeight int, keypair *ZcashKeypair) (*t2z.TransparentInput, error) {
	blockHash, err := client.GetBlockHash(blockHeight)
	if err != nil {
		return nil, err
	}

	blockData, err := client.GetBlock(blockHash, 2) // verbosity 2 for tx data
	if err != nil {
		return nil, err
	}

	var block struct {
		Tx []struct {
			Hex string `json:"hex"`
		} `json:"tx"`
	}
	if err := json.Unmarshal(blockData, &block); err != nil {
		return nil, err
	}

	if len(block.Tx) == 0 {
		return nil, nil
	}

	coinbaseTx := block.Tx[0]
	if coinbaseTx.Hex == "" {
		return nil, nil
	}

	outputs, err := ParseTxOutputs(coinbaseTx.Hex)
	if err != nil {
		return nil, err
	}

	expectedPubkeyHash := Hash160(keypair.PublicKey)

	for index, output := range outputs {
		// Check if this is a P2PKH output matching our pubkey
		// P2PKH: OP_DUP OP_HASH160 <20 bytes> OP_EQUALVERIFY OP_CHECKSIG
		if len(output.ScriptPubKey) == 25 &&
			output.ScriptPubKey[0] == 0x76 &&
			output.ScriptPubKey[1] == 0xa9 &&
			output.ScriptPubKey[2] == 0x14 &&
			output.ScriptPubKey[23] == 0x88 &&
			output.ScriptPubKey[24] == 0xac {

			pubkeyHashInScript := output.ScriptPubKey[3:23]
			if bytes.Equal(pubkeyHashInScript, expectedPubkeyHash) {
				txidHex, err := ComputeTxid(coinbaseTx.Hex)
				if err != nil {
					return nil, err
				}
				txidBytes, _ := HexToBytes(txidHex)
				txidReversed := ReverseBytes(txidBytes)

				var txid [32]byte
				copy(txid[:], txidReversed)

				return &t2z.TransparentInput{
					Pubkey:       keypair.PublicKey,
					TxID:         txid,
					Vout:         uint32(index),
					Amount:       output.Value,
					ScriptPubKey: output.ScriptPubKey,
				}, nil
			}
		}
	}

	return nil, nil
}

// GetMatureCoinbaseUtxos gets mature coinbase UTXOs (100+ confirmations)
func GetMatureCoinbaseUtxos(client *ZebraClient, keypair *ZcashKeypair, maxCount int) ([]t2z.TransparentInput, error) {
	info, err := client.GetBlockchainInfo()
	if err != nil {
		return nil, err
	}

	currentHeight := info.Blocks
	matureHeight := currentHeight - 100

	spentUtxos := LoadSpentUtxos()
	var utxos []t2z.TransparentInput

	// Scan from most recent mature blocks backwards
	for height := matureHeight; height >= 1 && len(utxos) < maxCount; height-- {
		utxo, err := GetCoinbaseUtxo(client, height, keypair)
		if err != nil {
			continue
		}
		if utxo != nil {
			key := fmt.Sprintf("%s:%d", BytesToHex(utxo.TxID[:]), utxo.Vout)
			if !spentUtxos[key] {
				utxos = append(utxos, *utxo)
			}
		}
	}

	return utxos, nil
}

// PrintWorkflowSummary prints a transaction workflow summary
func PrintWorkflowSummary(title string, inputs []t2z.TransparentInput, outputs []struct {
	Address string
	Amount  uint64
}, fee uint64) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println(title)
	fmt.Println(strings.Repeat("=", 70))

	var totalInput uint64
	for _, input := range inputs {
		totalInput += input.Amount
	}

	var totalOutput uint64
	for _, output := range outputs {
		totalOutput += output.Amount
	}

	fmt.Printf("\nInputs: %d\n", len(inputs))
	for i, input := range inputs {
		fmt.Printf("  [%d] %s ZEC\n", i, ZatoshiToZec(input.Amount))
	}
	fmt.Printf("  Total: %s ZEC\n", ZatoshiToZec(totalInput))

	fmt.Printf("\nOutputs: %d\n", len(outputs))
	for i, output := range outputs {
		addrShort := output.Address
		if len(addrShort) > 20 {
			addrShort = addrShort[:20]
		}
		fmt.Printf("  [%d] %s... -> %s ZEC\n", i, addrShort, ZatoshiToZec(output.Amount))
	}
	fmt.Printf("  Total: %s ZEC\n", ZatoshiToZec(totalOutput))

	fmt.Printf("\nFee: %s ZEC\n", ZatoshiToZec(fee))
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
}

// PrintBroadcastResult prints the broadcast result
func PrintBroadcastResult(txid string, txHex string) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("TRANSACTION BROADCAST SUCCESSFUL")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("\nTXID: %s\n", txid)
	if txHex != "" {
		fmt.Printf("\nRaw Transaction (%d bytes):\n", len(txHex)/2)
		if len(txHex) > 100 {
			fmt.Printf("%s...\n", txHex[:100])
		} else {
			fmt.Println(txHex)
		}
	}
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
}

// PrintError prints an error
func PrintError(title string, err error) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("ERROR: %s\n", title)
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("\nError: %v\n", err)
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
}

// TestData represents saved test data
type TestData struct {
	Transparent TransparentData `json:"transparent"`
	Network     string          `json:"network"`
	SetupHeight int             `json:"setupHeight"`
	SetupAt     string          `json:"setupAt"`
}

// TransparentData represents transparent address data
type TransparentData struct {
	Address    string `json:"address"`
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
	WIF        string `json:"wif"`
}

// LoadTestData loads test data from file
func LoadTestData() (*TestData, error) {
	filePath := filepath.Join(dataDir, "test-addresses.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var testData TestData
	if err := json.Unmarshal(data, &testData); err != nil {
		return nil, err
	}
	return &testData, nil
}

// SaveTestData saves test data to file
func SaveTestData(testData *TestData) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(testData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dataDir, "test-addresses.json"), data, 0644)
}
