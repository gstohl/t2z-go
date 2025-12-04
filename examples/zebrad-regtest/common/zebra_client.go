// Package common provides shared utilities for t2z regtest examples
package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ZebraClient is a JSON-RPC client for Zebra
type ZebraClient struct {
	url       string
	client    *http.Client
	idCounter int
}

// BlockchainInfo represents the response from getblockchaininfo
type BlockchainInfo struct {
	Chain                string  `json:"chain"`
	Blocks               int     `json:"blocks"`
	Headers              int     `json:"headers"`
	BestBlockHash        string  `json:"bestblockhash"`
	Difficulty           float64 `json:"difficulty"`
	VerificationProgress float64 `json:"verificationprogress"`
}

// Block represents a block from getblock
type Block struct {
	Hash   string          `json:"hash"`
	Height int             `json:"height"`
	Tx     json.RawMessage `json:"tx"`
}

// rpcRequest represents a JSON-RPC request
type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// rpcResponse represents a JSON-RPC response
type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
	ID     int             `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewZebraClient creates a new Zebra RPC client
func NewZebraClient() *ZebraClient {
	host := os.Getenv("ZEBRA_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("ZEBRA_PORT")
	if port == "" {
		port = "18232"
	}

	return &ZebraClient{
		url: fmt.Sprintf("http://%s:%s", host, port),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// rawCall makes a raw JSON-RPC call
func (c *ZebraClient) rawCall(method string, params ...interface{}) (json.RawMessage, error) {
	c.idCounter++

	if params == nil {
		params = []interface{}{}
	}

	reqBody := rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      c.idCounter,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.client.Post(c.url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error: %s", rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// WaitForReady waits for Zebra to be ready
func (c *ZebraClient) WaitForReady(maxAttempts int, delayMs int) error {
	if maxAttempts == 0 {
		maxAttempts = 30
	}
	if delayMs == 0 {
		delayMs = 1000
	}

	for i := 0; i < maxAttempts; i++ {
		_, err := c.GetBlockchainInfo()
		if err == nil {
			return nil
		}
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}
	return fmt.Errorf("node not ready after %d attempts", maxAttempts)
}

// GetBlockchainInfo returns blockchain info
func (c *ZebraClient) GetBlockchainInfo() (*BlockchainInfo, error) {
	result, err := c.rawCall("getblockchaininfo")
	if err != nil {
		return nil, err
	}

	var info BlockchainInfo
	if err := json.Unmarshal(result, &info); err != nil {
		return nil, fmt.Errorf("unmarshal blockchain info: %w", err)
	}
	return &info, nil
}

// GetBlockCount returns the current block count
func (c *ZebraClient) GetBlockCount() (int, error) {
	result, err := c.rawCall("getblockcount")
	if err != nil {
		return 0, err
	}

	var count int
	if err := json.Unmarshal(result, &count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetBlockHash returns the block hash at the given height
func (c *ZebraClient) GetBlockHash(height int) (string, error) {
	result, err := c.rawCall("getblockhash", height)
	if err != nil {
		return "", err
	}

	var hash string
	if err := json.Unmarshal(result, &hash); err != nil {
		return "", err
	}
	return hash, nil
}

// GetBlock returns block data
func (c *ZebraClient) GetBlock(hash string, verbosity int) (json.RawMessage, error) {
	result, err := c.rawCall("getblock", hash, verbosity)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// SendRawTransaction broadcasts a raw transaction
func (c *ZebraClient) SendRawTransaction(txHex string) (string, error) {
	result, err := c.rawCall("sendrawtransaction", txHex)
	if err != nil {
		return "", err
	}

	var txid string
	if err := json.Unmarshal(result, &txid); err != nil {
		return "", err
	}
	return txid, nil
}

// WaitForBlocks waits until the specified block height is reached
func (c *ZebraClient) WaitForBlocks(targetHeight int, timeoutMs int) (int, error) {
	if timeoutMs == 0 {
		timeoutMs = 120000
	}

	startTime := time.Now()
	lastHeight := 0

	for {
		if time.Since(startTime) > time.Duration(timeoutMs)*time.Millisecond {
			return 0, fmt.Errorf("timeout waiting for height %d", targetHeight)
		}

		info, err := c.GetBlockchainInfo()
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		if info.Blocks >= targetHeight {
			return info.Blocks, nil
		}

		if info.Blocks != lastHeight {
			lastHeight = info.Blocks
			fmt.Printf("  Block height: %d\n", info.Blocks)
		}

		time.Sleep(1 * time.Second)
	}
}
