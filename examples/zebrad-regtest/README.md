# Go Zebrad Examples

Go examples demonstrating t2z library usage with Zebra regtest.

## Quick Start

**Requirements:**
- Go 1.21+
- Docker and docker-compose
- Rust toolchain (for building native library)

**Important:** Reset docker before running examples to ensure fresh coinbase UTXOs:

```bash
# From repository root
cd infra/zebrad-regtest
docker-compose down -v && docker-compose up -d

# Run setup (waits for block 101)
cd ../../bindings/go/examples/zebrad-regtest
go run ./setup

# Run Go examples
go run ./1-single-output
```

## Prerequisites

1. **Zebra regtest running** (via docker-compose in `infra/zebrad-regtest/`)
2. **Go library built** (run `go build` in `bindings/go/`)
3. **Rust library built** (run `cargo build --release` in `core/rust/`)

## Running Examples

```bash
cd bindings/go/examples/zebrad-regtest

# Run individual examples
go run ./1-single-output      # Single transparent output (T→T)
go run ./2-multiple-outputs   # Multiple transparent outputs (T→T×2)
go run ./3-utxo-consolidation # UTXO consolidation (2 inputs → 1 output)
go run ./4-attack-scenario    # Attack detection - PCZT verification
go run ./5-shielded-output    # Single shielded output (T→Z)
go run ./6-multiple-shielded  # Multiple shielded outputs (T→Z×2)
go run ./7-mixed-outputs      # Mixed transparent + shielded (T→T+Z)
go run ./8-combine-workflow   # Combine workflow (parallel signing)
go run ./9-offline-signing    # Offline signing (hardware wallet)
```

## Examples Overview

| Example | Description |
|---------|-------------|
| 1 | Single output transparent transaction |
| 2 | Multiple recipients (2 transparent outputs) |
| 3 | UTXO consolidation (multiple inputs → single output) |
| 4 | Attack detection - verifying PCZT before signing |
| 5 | Transparent to shielded (Orchard) |
| 6 | Multiple shielded recipients |
| 7 | Mixed transparent + shielded outputs |
| 8 | Combine workflow - parallel signing by multiple parties |
| 9 | Offline signing - hardware wallet / air-gapped device |

## Project Structure

```
go/examples/zebrad-regtest/
├── 1-single-output/main.go
├── 2-multiple-outputs/main.go
├── 3-utxo-consolidation/main.go
├── 4-attack-scenario/main.go
├── 5-shielded-output/main.go
├── 6-multiple-shielded/main.go
├── 7-mixed-outputs/main.go
├── 8-combine-workflow/main.go
├── 9-offline-signing/main.go
└── README.md
```
