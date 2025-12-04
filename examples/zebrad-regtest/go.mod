module github.com/gstohl/t2z/go/examples/zebrad-regtest

go 1.24.0

require (
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1
	github.com/gstohl/t2z/go v0.0.0
	golang.org/x/crypto v0.45.0
)

replace github.com/gstohl/t2z/go => ../..
