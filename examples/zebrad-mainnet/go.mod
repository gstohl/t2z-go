module zebrad-mainnet

go 1.24.0

require (
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.3.0
	golang.org/x/crypto v0.31.0
	t2z v0.0.0
)

replace t2z => ../..
