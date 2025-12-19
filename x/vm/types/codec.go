package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/gogoproto/proto"

	"github.com/cosmos/evm/x/vm/types/legacy"
)

var (
	amino = codec.NewLegacyAmino()
	// ModuleCdc references the global evm module codec. Note, the codec should
	// ONLY be used in certain instances of tests and for JSON encoding.
	ModuleCdc = codec.NewProtoCodec(codectypes.NewInterfaceRegistry())

	// AminoCdc is a amino codec created to support amino JSON compatible msgs.
	AminoCdc = codec.NewLegacyAmino()
)

const (
	// Amino names
	updateParamsName = "os/evm/MsgUpdateParams"
)

// NOTE: This is required for the GetSignBytes function
func init() {
	RegisterLegacyAminoCodec(amino)
	amino.Seal()

	// Register legacy types with cosmos.evm.vm.v1 namespace for backward compatibility
	// This is needed because old transactions may have these type URLs
	proto.RegisterType((*legacy.DynamicFeeTx)(nil), "cosmos.evm.vm.v1.DynamicFeeTx")
	proto.RegisterType((*legacy.AccessListTx)(nil), "cosmos.evm.vm.v1.AccessListTx")
	proto.RegisterType((*legacy.LegacyTx)(nil), "cosmos.evm.vm.v1.LegacyTx")
	proto.RegisterType((*legacy.MsgEthereumTx)(nil), "cosmos.evm.vm.v1.MsgEthereumTx")
	proto.RegisterType((*legacy.ExtensionOptionsEthereumTx)(nil), "cosmos.evm.vm.v1.ExtensionOptionsEthereumTx")
}

// RegisterInterfaces registers the client interfaces to protobuf Any.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations(
		(*tx.TxExtensionOptionI)(nil),
		&ExtensionOptionsEthereumTx{},
		&legacy.ExtensionOptionsEthereumTx{}, // Legacy extension option
	)
	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&MsgEthereumTx{},
		&MsgUpdateParams{},
		&legacy.MsgEthereumTx{}, // Legacy MsgEthereumTx
	)

	// Register TxData implementations for unpacking legacy Any field
	// These are registered under multiple namespaces for backward compatibility
	registry.RegisterInterface(
		"ethermint.evm.v1.TxData",
		(*legacy.TxData)(nil),
		&legacy.DynamicFeeTx{},
		&legacy.AccessListTx{},
		&legacy.LegacyTx{},
	)
	registry.RegisterInterface(
		"os.evm.v1.TxData",
		(*legacy.TxData)(nil),
		&legacy.DynamicFeeTx{},
		&legacy.AccessListTx{},
		&legacy.LegacyTx{},
	)
	registry.RegisterInterface(
		"cosmos.evm.vm.v1.TxData",
		(*legacy.TxData)(nil),
		&legacy.DynamicFeeTx{},
		&legacy.AccessListTx{},
		&legacy.LegacyTx{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

// RegisterLegacyAminoCodec required for EIP-712
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgUpdateParams{}, updateParamsName, nil)
}