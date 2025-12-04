# Mainnet Examples - Go

Interactive examples for sending ZEC on mainnet with memo support and hardware wallet simulation.

## Prerequisites

1. Zebra mainnet node running and synced (`infra/zebrad-mainnet/`)
2. Real ZEC in your wallet
3. Rust library built (`core/rust/`)
4. Go bindings built (`bindings/go/`)

## Scripts

### 1. Generate Wallet

Creates a new wallet and saves credentials to `.env`:

```bash
go run ./cmd/generate-wallet
```

Output:
```
New wallet generated!

Address: t1XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX

Saved to: /path/to/.env

IMPORTANT: Back up your private key securely!
```

### 2. Interactive Send

Send ZEC to any address (transparent or shielded) with optional memo:

```bash
go run ./cmd/send
```

Features:
- Multiple recipients support
- Memo support for shielded addresses (up to 512 bytes)
- Automatic fee calculation (ZIP-317)
- Balance display

### 3. Hardware Wallet Simulation

Demonstrates offline signing workflow using two devices:

**Device A** (Online) - Builds transaction, outputs sighash:
```bash
go run ./cmd/device-a
```

**Device B** (Offline) - Signs sighash with private key:
```bash
go run ./cmd/device-b
```

Workflow:
1. Run `device-a`, enter recipient and amount
2. Copy the SIGHASH to Device B
3. Run `device-b`, paste the sighash
4. Copy the SIGNATURE back to Device A
5. Device A broadcasts the transaction

This simulates how hardware wallets work - the private key never leaves Device B!

## Environment Variables

The `.env` file contains:
```
PRIVATE_KEY=<hex>
PUBLIC_KEY=<hex>
ADDRESS=<t1...>
ZEBRA_HOST=localhost
ZEBRA_PORT=8232
```

## Fee Calculation

Fees are calculated using ZIP-317:
- Transparent-only: ~10,000 zatoshis
- With shielded output: ~15,000 zatoshis
- Memos don't affect the fee
