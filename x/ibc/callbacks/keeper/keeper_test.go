package keeper

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/ibc/callbacks/types"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	callbacktypes "github.com/cosmos/ibc-go/v10/modules/apps/callbacks/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	sdktestutil "github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func ensureBech32Config(t *testing.T) {
	t.Helper()
	cfg := sdk.GetConfig()
	if cfg.GetBech32AccountAddrPrefix() != "" {
		return
	}
	cfg.SetBech32PrefixForAccount("cosmos", "cosmospub")
	cfg.SetBech32PrefixForValidator("cosmosvaloper", "cosmosvaloperpub")
	cfg.SetBech32PrefixForConsensusNode("cosmosvalcons", "cosmosvalconspub")
	cfg.Seal()
}

// outOfGasEVMKeeper simulates an EVM callback that burns all of its gas.
type outOfGasEVMKeeper struct {
	types.EVMKeeper
	callEVM func(ctx sdk.Context, gasCap *big.Int) (*evmtypes.MsgEthereumTxResponse, error)
}

func (outOfGasEVMKeeper) IsContract(ctx sdk.Context, _ common.Address) bool {
	// mimic the KV read gas charged on the callback context
	ctx.GasMeter().ConsumeGas(1_000, "is contract")
	return true
}

func (k outOfGasEVMKeeper) CallEVM(ctx sdk.Context, _ *statedb.StateDB, _ abi.ABI, _, _ common.Address, _, _ bool, gasCap *big.Int, _ string, _ ...interface{}) (*evmtypes.MsgEthereumTxResponse, error) {
	return k.callEVM(ctx, gasCap)
}

func TestSourceCallbacks_ChargeGasOnFailedEVMCall(t *testing.T) {
	ensureBech32Config(t)
	contractEth := common.HexToAddress("0x5555555555555555555555555555555555555555")
	senderBech32 := sdk.AccAddress(contractEth.Bytes()).String()
	memoBz, err := json.Marshal(map[string]any{
		callbacktypes.SourceCallbackKey: map[string]string{
			"address":   contractEth.Hex(),
			"gas_limit": "1000000",
		},
	})
	require.NoError(t, err)
	packetData := transfertypes.NewFungibleTokenPacketData("stake", "1", senderBech32, senderBech32, string(memoBz))
	packetDataBz, err := transfertypes.MarshalPacketData(packetData, transfertypes.V1, transfertypes.EncodingJSON)
	require.NoError(t, err)
	packet := channeltypes.NewPacket(packetDataBz, 1, "transfer", "channel-0", "transfer", "channel-1", clienttypes.NewHeight(0, 100), 0)

	storeKey := storetypes.NewKVStoreKey("test")
	settledKey := []byte("settled")
	successfulCall := func(ctx sdk.Context, _ *big.Int) (*evmtypes.MsgEthereumTxResponse, error) {
		ctx.KVStore(storeKey).Set(settledKey, []byte{1})
		return &evmtypes.MsgEthereumTxResponse{GasUsed: 50_000}, nil
	}

	callbacks := []struct {
		name   string
		invoke func(k ContractKeeper, ctx sdk.Context) error
	}{
		{"acknowledgement", func(k ContractKeeper, ctx sdk.Context) error {
			return k.IBCOnAcknowledgementPacketCallback(ctx, packet, []byte("ack"), sdk.AccAddress{}, contractEth.Hex(), senderBech32, transfertypes.V1)
		}},
		{"timeout", func(k ContractKeeper, ctx sdk.Context) error {
			return k.IBCOnTimeoutPacketCallback(ctx, packet, sdk.AccAddress{}, contractEth.Hex(), senderBech32, transfertypes.V1)
		}},
	}

	testCases := []struct {
		name string
		// relayer-provided execution gas
		gasLimit uint64
		callEVM  func(ctx sdk.Context, gasCap *big.Int) (*evmtypes.MsgEthereumTxResponse, error)
		// expPastLimit is true if IBC-Go must revert the packet lifecycle for a retry
		expPastLimit bool
		// expConsumed is the gas expected on the callback gas meter if not past its limit
		expConsumed uint64
		// expSettled is true if the callback must succeed and commit its state
		expSettled bool
	}{
		{
			name:     "vm out of gas",
			gasLimit: 200_000,
			callEVM: func(_ sdk.Context, gasCap *big.Int) (*evmtypes.MsgEthereumTxResponse, error) {
				res := &evmtypes.MsgEthereumTxResponse{GasUsed: gasCap.Uint64(), VmError: vm.ErrOutOfGas.Error()}
				return res, errorsmod.Wrap(evmtypes.ErrVMExecution, res.VmError)
			},
			expPastLimit: true,
		},
		{
			name:     "vm out of gas using exactly the remaining callback gas",
			gasLimit: 200_000,
			callEVM: func(_ sdk.Context, gasCap *big.Int) (*evmtypes.MsgEthereumTxResponse, error) {
				// the call gas cap is snapshotted before the IsContract read is charged
				res := &evmtypes.MsgEthereumTxResponse{GasUsed: gasCap.Uint64() - 1_000, VmError: vm.ErrOutOfGas.Error()}
				return res, errorsmod.Wrap(evmtypes.ErrVMExecution, res.VmError)
			},
			expPastLimit: true,
		},
		{
			name:     "insufficient intrinsic gas",
			gasLimit: 20_000,
			callEVM: func(_ sdk.Context, _ *big.Int) (*evmtypes.MsgEthereumTxResponse, error) {
				return nil, errorsmod.Wrap(core.ErrIntrinsicGas, "apply message")
			},
			expPastLimit: true,
		},
		{
			name:     "insufficient floor data gas",
			gasLimit: 20_000,
			callEVM: func(_ sdk.Context, gasCap *big.Int) (*evmtypes.MsgEthereumTxResponse, error) {
				return nil, fmt.Errorf("%w: have %d, want %d", core.ErrFloorDataGas, gasCap.Uint64(), 25_000)
			},
			expPastLimit: true,
		},
		{
			name:     "nil response with execution gas past the limit",
			gasLimit: 200_000,
			callEVM: func(ctx sdk.Context, gasCap *big.Int) (*evmtypes.MsgEthereumTxResponse, error) {
				// mimic the StateDB commit error branch of ApplyMessageWithConfig
				ctx.GasMeter().ConsumeGas(gasCap.Uint64(), "apply evm message")
				return nil, errorsmod.Wrap(errors.New("commit failed"), "failed to commit stateDB")
			},
			expPastLimit: true,
		},
		{
			name:     "nil response with recorded execution gas",
			gasLimit: 200_000,
			callEVM: func(ctx sdk.Context, _ *big.Int) (*evmtypes.MsgEthereumTxResponse, error) {
				ctx.GasMeter().ConsumeGas(50_000, "apply evm message")
				return nil, errorsmod.Wrap(errors.New("commit failed"), "failed to commit stateDB")
			},
			expConsumed: 1_000 + 50_000,
		},
		{
			name:        "adequately funded relay",
			gasLimit:    10_000_000,
			callEVM:     successfulCall,
			expConsumed: 1_000 + 50_000,
			expSettled:  true,
		},
		{
			name:        "successful call using exactly the remaining callback gas",
			gasLimit:    1_000 + 50_000,
			callEVM:     successfulCall,
			expConsumed: 1_000 + 50_000,
			expSettled:  true,
		},
	}

	for _, tc := range testCases {
		for _, cb := range callbacks {
			t.Run(tc.name+"/"+cb.name, func(t *testing.T) {
				k := ContractKeeper{evmKeeper: outOfGasEVMKeeper{callEVM: tc.callEVM}}
				ctx := sdktestutil.DefaultContext(storeKey, storetypes.NewTransientStoreKey("test_t")).
					WithLogger(log.NewNopLogger()).
					WithGasMeter(storetypes.NewGasMeter(tc.gasLimit))

				var err error
				func() {
					defer func() {
						if r := recover(); r != nil {
							_, ok := r.(storetypes.ErrorOutOfGas)
							require.True(t, ok, "unexpected panic: %v", r)
						}
					}()
					err = cb.invoke(k, ctx)
				}()

				if tc.expSettled {
					require.NoError(t, err)
				}
				settled := ctx.WithGasMeter(storetypes.NewInfiniteGasMeter()).KVStore(storeKey).Has(settledKey)
				require.Equal(t, tc.expSettled, settled)

				// IBC-Go only reverts the packet lifecycle for a retry when the
				// callback gas meter is past its limit.
				if tc.expPastLimit {
					require.True(t, ctx.GasMeter().IsPastLimit())
				} else {
					require.False(t, ctx.GasMeter().IsPastLimit())
					require.Equal(t, tc.expConsumed, ctx.GasMeter().GasConsumed())
				}
			})
		}
	}
}
