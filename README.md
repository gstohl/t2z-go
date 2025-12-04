# t2z-go

> **Auto-generated** - Do not edit directly. All changes must be made in [gstohl/t2z](https://github.com/gstohl/t2z).

Go bindings for t2z - enabling transparent Zcash wallets to send shielded Orchard outputs via PCZT ([ZIP 374](https://zips.z.cash/zip-0374)).

## Installation

```bash
go get github.com/gstohl/t2z-go
```

Native libraries are bundled for: macOS (arm64/x64), Linux (x64/arm64), Windows (x64/arm64).

## Usage

```go
package main

import (
    "log"
    t2z "github.com/gstohl/t2z-go"
)

func main() {
    // 1. Create payment request
    request, _ := t2z.NewTransactionRequest([]t2z.Payment{{
        Address: "u1...",    // unified or transparent address
        Amount:  100000,     // 0.001 ZEC in zatoshis
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
    signature := t2z.SignMessage(privateKey, sighash[:])

    signed, _ := t2z.AppendSignature(proved, 0, signature)

    // 5. Finalize and broadcast
    txBytes, _ := t2z.FinalizeAndExtract(signed)
    // submit txBytes to Zcash network
}
```

## API

See the [main repo](https://github.com/gstohl/t2z) for full documentation.

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
| `SignMessage` | secp256k1 signing utility |
| `GetPublicKey` | Derive compressed public key |
| `CalculateFee` | Calculate ZIP-317 fee |

## Types

```go
type Payment struct {
    Address string   // transparent (t1...) or unified (u1...)
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

## Examples

See [examples/](examples/) for complete working examples:

- **zebrad-regtest/** - Local regtest network examples (1-9)
- **zebrad-mainnet/** - Mainnet examples with hardware wallet flow

## Memory

**Automatic cleanup**: All handles are automatically freed by the garbage collector via `runtime.SetFinalizer`. No manual cleanup required.

## License

MIT
