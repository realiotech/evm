package types

const (
	// ModuleName string name of module
	ModuleName = "feesponsor"

	// StoreKey key for the feesponsor module store
	StoreKey = ModuleName

	// RouterKey uses module name for routing
	RouterKey = ModuleName
)

// prefix bytes for the feesponsor persistent store
const (
	prefixFeePayer = iota + 1
)

// KVStore key prefixes
var (
	KeyPrefixFeePayer = []byte{prefixFeePayer}
)

