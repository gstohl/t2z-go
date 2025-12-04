# t2z Go Bindings

Go bindings using CGO to wrap the Rust core library.

## Installation

```bash
# Build Rust library first
cd ../../core/rust && cargo build --release

# Test Go bindings
cd ../../bindings/go && go test -v
```

## Usage

```go
package main

import (
    "log"
    t2z "github.com/gstohl/t2z/go"
    "github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

func main() {
    // 1. Create payment request
    request, _ := t2z.NewTransactionRequest([]t2z.Payment{{
        Address: "utest1...",  // unified or transparent address
        Amount:  100000,       // 0.001 ZEC in zatoshis
    }})
    defer request.Free()

    // 2. Create PCZT from transparent UTXOs
    inputs := []t2z.TransparentInput{{
        Pubkey:       pubkeyBytes,    // 33 bytes compressed
        TxID:         txidBytes,      // 32 bytes
        Vout:         0,
        Amount:       100000000,      // 1 ZEC
        ScriptPubKey: scriptBytes,
    }}

    pczt, _ := t2z.ProposeTransaction(inputs, request)

    // 3. Add Orchard proofs
    proved, _ := t2z.ProveTransaction(pczt)

    // 4. Sign transparent inputs
    sighash, _ := t2z.GetSighash(proved, 0)
    sig := ecdsa.SignCompact(privKey, sighash[:], false)
    var signature [64]byte
    copy(signature[:], sig[1:]) // skip recovery byte

    signed, _ := t2z.AppendSignature(proved, 0, signature)

    // 5. Finalize and broadcast
    txBytes, _ := t2z.FinalizeAndExtract(signed)
    // submit txBytes to zcashd/lightwalletd
}
```

## API

See [root README](../README.md) for full API documentation.

| Function | Description |
|----------|-------------|
| `NewTransactionRequest` | Create payment request |
| `ProposeTransaction` | Create PCZT from inputs |
| `ProveTransaction` | Add Orchard proofs |
| `VerifyBeforeSigning` | Verify PCZT integrity |
| `GetSighash` | Get signature hash for input |
| `AppendSignature` | Add 64-byte signature |
| `Combine` | Merge multiple PCZTs |
| `FinalizeAndExtract` | Extract transaction bytes |
| `ParsePCZT` / `SerializePCZT` | PCZT serialization |

## Types

```go
type Payment struct {
    Address string   // transparent (tm...) or unified (utest1...)
    Amount  uint64   // zatoshis
    Memo    string   // optional, for shielded outputs
}

type TransparentInput struct {
    Pubkey       []byte   // 33 bytes compressed secp256k1
    TxID         [32]byte
    Vout         uint32
    Amount       uint64   // zatoshis
    ScriptPubKey []byte   // P2PKH script
}
```

## Memory

**Automatic cleanup**: All handles are automatically freed by the garbage collector via `runtime.SetFinalizer`. No manual cleanup required.

Consuming functions transfer ownership (input PCZT becomes invalid):
- `ProveTransaction`, `AppendSignature`, `FinalizeAndExtract`, `Combine`

Non-consuming (read-only):
- `GetSighash`, `Serialize`, `VerifyBeforeSigning`

## License

MIT
