package types

type CallType int

const (
	// RPC call type is used on requests to eth_estimateGas rpc API endpoint
	RPC CallType = iota + 1
	// Internal call type is used in case of smart contract methods calls
	Internal
)

// MaxPrecompileCalls is the maximum number of precompile calls
// issued by a top-level StateDB and every StateDB nested in its
// precompile calls. We want to limit this because for each
// precompile tx we're creating a cached context. A StateDB built
// from a context that carries no limit is top-level and starts
// its own count, so a Cosmos transaction that builds several
// top-level StateDBs gets a separate limit for each of them.
const MaxPrecompileCalls uint64 = 20

const (
	// MaxLogsPerTx is the maximum number of logs a single EVM message may
	// retain. It is above the 44,739 LOG0 operations a transaction can
	// afford under the EIP-7825 gas cap.
	MaxLogsPerTx = 65_536
	// MaxLogDataBytesPerTx is the maximum aggregate log data size, in bytes,
	// a single EVM message may retain. It is above the 2 MiB of log data a
	// transaction can pay for under the EIP-7825 gas cap.
	MaxLogDataBytesPerTx = 4 * 1024 * 1024
)
