package types

import (
	"crypto/sha256"

	"github.com/ethereum/go-ethereum/common"
)

const (
	// ModuleName string name of module
	ModuleName = "evm"

	// StoreKey key for ethereum storage data, account code (StateDB) or block
	// related data for Web3.
	// The EVM module should use a prefix store.
	StoreKey = ModuleName

	// TransientKey is the key to access the EVM transient store, that is reset
	// during the Commit phase.
	TransientKey = "transient_" + ModuleName

	// RouterKey uses module name for routing
	RouterKey = ModuleName
)

// prefix bytes for the EVM persistent store
const (
	prefixCode = iota + 1
	prefixStorage
	prefixParams
	prefixCodeHash
	prefixEvmCoinInfo
)

// prefix bytes for the EVM transient store
const (
	prefixTransientBloom = iota + 1
	prefixTransientTxIndex
	prefixTransientLogSize
	prefixTransientGasUsed
	prefixTransientAnteVerifiedSender
)

// KVStore key prefixes
var (
	KeyPrefixCode        = []byte{prefixCode}
	KeyPrefixStorage     = []byte{prefixStorage}
	KeyPrefixParams      = []byte{prefixParams}
	KeyPrefixCodeHash    = []byte{prefixCodeHash}
	KeyPrefixEvmCoinInfo = []byte{prefixEvmCoinInfo}
)

// Transient Store key prefixes
var (
	KeyPrefixTransientBloom              = []byte{prefixTransientBloom}
	KeyPrefixTransientTxIndex            = []byte{prefixTransientTxIndex}
	KeyPrefixTransientLogSize            = []byte{prefixTransientLogSize}
	KeyPrefixTransientGasUsed            = []byte{prefixTransientGasUsed}
	KeyPrefixTransientAnteVerifiedSender = []byte{prefixTransientAnteVerifiedSender}
)

// AddressStoragePrefix returns a prefix to iterate over a given account storage.
func AddressStoragePrefix(address common.Address) []byte {
	return append(KeyPrefixStorage, address.Bytes()...)
}

// StateKey defines the full key under which an account state is stored.
func StateKey(address common.Address, key []byte) []byte {
	return append(AddressStoragePrefix(address), key...)
}

// TransientAnteVerifiedSenderKey returns the transient store key under which
// the sender verified by the EVM ante handler is recorded for the Ethereum tx
// with the given hash, delivered in the Cosmos tx with the given bytes.
func TransientAnteVerifiedSenderKey(cosmosTxBytes []byte, ethTxHash common.Hash) []byte {
	cosmosTxHash := sha256.Sum256(cosmosTxBytes)

	var anteVerifiedSenderKey [1 + sha256.Size + common.HashLength]byte

	anteVerifiedSenderKey[0] = prefixTransientAnteVerifiedSender
	copy(anteVerifiedSenderKey[1:], cosmosTxHash[:])
	copy(anteVerifiedSenderKey[1+sha256.Size:], ethTxHash.Bytes())

	return anteVerifiedSenderKey[:]
}
