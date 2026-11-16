package cosmos_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/ante/cosmos"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// mockMsgRouter returns a handler for every msg type.
type mockMsgRouter struct{}

func (mockMsgRouter) Handler(_ sdk.Msg) baseapp.MsgServiceHandler {
	return func(_ sdk.Context, _ sdk.Msg) (*sdk.Result, error) { return &sdk.Result{}, nil }
}

func (mockMsgRouter) HandlerByTypeURL(_ string) baseapp.MsgServiceHandler {
	return func(_ sdk.Context, _ sdk.Msg) (*sdk.Result, error) { return &sdk.Result{}, nil }
}

func TestDenyListMsgRouter(t *testing.T) {
	router := cosmos.NewDenyListMsgRouter(
		mockMsgRouter{},
		sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{}),
		sdk.MsgTypeURL(&sdkvesting.MsgCreateVestingAccount{}),
	)

	for _, msg := range []sdk.Msg{&evmtypes.MsgEthereumTx{}, &sdkvesting.MsgCreateVestingAccount{}} {
		require.Nil(t, router.Handler(msg))
		require.Nil(t, router.HandlerByTypeURL(sdk.MsgTypeURL(msg)))
	}

	require.NotNil(t, router.Handler(&banktypes.MsgSend{}))
	require.NotNil(t, router.HandlerByTypeURL(sdk.MsgTypeURL(&banktypes.MsgSend{})))
}
