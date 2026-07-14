package legacy

import (
	"fmt"
	"sync"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/gogoproto/proto"
)

// customTypeURLRegistry is implemented by registries that support custom type URL registration
type customTypeURLRegistry interface {
	RegisterCustomTypeURL(iface any, typeURL string, impl proto.Message)
}

// legacyTxConfig holds the transaction config for decoding legacy transactions
var (
	legacyTxConfig client.TxConfig
	legacyOnce     sync.Once
)

// initLegacyTxConfig initializes the legacy tx config lazily.
// This must be called lazily (not in init()) to ensure all proto types
// are registered before we try to use them.
func initLegacyTxConfig() {
	legacyOnce.Do(func() {
		// Create interface registry with legacy type mappings
		interfaceRegistry := codectypes.NewInterfaceRegistry()

		// Register standard SDK types
		sdk.RegisterInterfaces(interfaceRegistry)

		// Register legacy EVM types
		interfaceRegistry.RegisterImplementations(
			(*tx.TxExtensionOptionI)(nil),
			&ExtensionOptionsEthereumTx{},
		)
		interfaceRegistry.RegisterImplementations(
			(*sdk.Msg)(nil),
			&MsgEthereumTx{},
			&MsgUpdateParams{},
		)

		// Register TxData types
		interfaceRegistry.RegisterInterface(
			"os.evm.v1.TxData",
			(*TxData)(nil),
			&DynamicFeeTx{},
			&AccessListTx{},
			&LegacyTx{},
		)
		interfaceRegistry.RegisterInterface(
			"ethermint.evm.v1.TxData",
			(*TxData)(nil),
			&DynamicFeeTx{},
			&AccessListTx{},
			&LegacyTx{},
		)
		interfaceRegistry.RegisterInterface(
			"cosmos.evm.vm.v1.TxData",
			(*TxData)(nil),
			&DynamicFeeTx{},
			&AccessListTx{},
			&LegacyTx{},
		)

		// Register custom type URLs for backward compatibility
		// This maps the old type URL to our legacy types
		if reg, ok := interfaceRegistry.(customTypeURLRegistry); ok {
			reg.RegisterCustomTypeURL((*sdk.Msg)(nil), "/cosmos.evm.vm.v1.MsgEthereumTx", &MsgEthereumTx{})
			reg.RegisterCustomTypeURL((*tx.TxExtensionOptionI)(nil), "/cosmos.evm.vm.v1.ExtensionOptionsEthereumTx", &ExtensionOptionsEthereumTx{})
			reg.RegisterCustomTypeURL((*TxData)(nil), "/cosmos.evm.vm.v1.DynamicFeeTx", &DynamicFeeTx{})
			reg.RegisterCustomTypeURL((*TxData)(nil), "/cosmos.evm.vm.v1.AccessListTx", &AccessListTx{})
			reg.RegisterCustomTypeURL((*TxData)(nil), "/cosmos.evm.vm.v1.LegacyTx", &LegacyTx{})
		}

		// Create codec and tx config
		protoCodec := codec.NewProtoCodec(interfaceRegistry)
		legacyTxConfig = authtx.NewTxConfig(protoCodec, authtx.DefaultSignModes)
	})
}

// DecodeTx decodes transaction bytes using the legacy-aware decoder.
// This handles transactions with the old MsgEthereumTx format.
func DecodeTx(txBytes []byte) (sdk.Tx, error) {
	// Lazily initialize the legacy tx config
	initLegacyTxConfig()

	if legacyTxConfig == nil {
		return nil, fmt.Errorf("legacy tx config not initialized")
	}
	return legacyTxConfig.TxDecoder()(txBytes)
}