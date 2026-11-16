package integration

import (
	"testing"

	"github.com/stretchr/testify/require"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/keyring"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

// TestMsgEthereumTxDispatchedByAnotherMsg checks that the MsgEthereumTx msg
// handler rejects a message that another message dispatches.
func TestMsgEthereumTxDispatchedByAnotherMsg(t *testing.T) {
	// The keepers under test are fields of the app, which the network only
	// exposes through an interface.
	var evmdApp *evmd.EVMD

	createCapturedEvmd := func(chainID string, evmChainID uint64, baseAppOptions ...func(*baseapp.BaseApp)) evm.EvmApp {
		createdApp := CreateEvmd(chainID, evmChainID, baseAppOptions...)
		evmdApp = createdApp.(*evmd.EVMD)

		return createdApp
	}

	testKeyring := keyring.New(2)
	preFundedAccounts := network.WithPreFundedAccounts(testKeyring.GetAllAccAddrs()...)
	unitNetwork := network.NewUnitTestNetwork(createCapturedEvmd, preFundedAccounts)
	require.NotNil(t, evmdApp)

	integrationHandler := grpc.NewIntegrationHandler(unitNetwork)
	txFactory := factory.New(unitNetwork, integrationHandler)

	rawSignerAccAddress := testKeyring.GetAccAddr(0)
	recipientAddress := testKeyring.GetAddr(1)
	govModuleAddress := authtypes.NewModuleAddress(govtypes.ModuleName)
	evmDenom := evmtypes.GetEVMCoinDenom()

	// newTransferMsg returns a transfer signed by the first key of the keyring
	// that names the given address as its sender.
	newTransferMsg := func(fromAddress []byte) *evmtypes.MsgEthereumTx {
		transferArgs := evmtypes.EvmTxArgs{To: &recipientAddress, GasLimit: 1_000_000}
		transferMsg, err := txFactory.GenerateSignedMsgEthereumTx(testKeyring.GetPrivKey(0), transferArgs)
		require.NoError(t, err)

		transferMsg.From = fromAddress

		return &transferMsg
	}

	// The handler refunds unused gas out of the fee collector's balance, so it
	// is funded for a refund to be possible at all.
	refundableFees := sdk.NewCoins(sdk.NewCoin(evmDenom, sdkmath.NewInt(1e18)))

	require.NoError(t, evmdApp.BankKeeper.MintCoins(unitNetwork.GetContext(), minttypes.ModuleName, refundableFees))
	require.NoError(t, evmdApp.BankKeeper.SendCoinsFromModuleToModule(
		unitNetwork.GetContext(), minttypes.ModuleName, authtypes.FeeCollectorName, refundableFees,
	))

	// The authz keeper looks the handler up in the app's msg router and calls
	// it without running the ante handlers.
	t.Run("msg handler rejects MsgEthereumTx executed through authz", func(t *testing.T) {
		// The grantee is the signer of the message, so no grant is needed.
		dispatchedMsgs := []sdk.Msg{newTransferMsg(rawSignerAccAddress)}
		signerBalanceBefore := evmdApp.BankKeeper.GetBalance(unitNetwork.GetContext(), rawSignerAccAddress, evmDenom)

		_, err := evmdApp.AuthzKeeper.DispatchActions(unitNetwork.GetContext(), rawSignerAccAddress, dispatchedMsgs)

		require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)

		signerBalanceAfter := evmdApp.BankKeeper.GetBalance(unitNetwork.GetContext(), rawSignerAccAddress, evmDenom)
		require.Equal(t, signerBalanceBefore, signerBalanceAfter, "a rejected message must not refund or charge the raw signer")
	})

	// Any other module that dispatches msgs (gov, group, the ICA host, a
	// contract module) reaches the handler through the same router.
	interchainAccountStyleAddress := make([]byte, 32)
	for byteIndex := range interchainAccountStyleAddress {
		interchainAccountStyleAddress[byteIndex] = 0xAA
	}

	directDispatchCases := []struct {
		name        string
		fromAddress []byte
		expectedErr error
	}{
		{
			name:        "from is the raw signer",
			fromAddress: rawSignerAccAddress,
			expectedErr: sdkerrors.ErrUnauthorized,
		},
		{
			name:        "from is another 20 byte address",
			fromAddress: govModuleAddress,
			expectedErr: sdkerrors.ErrUnauthorized,
		},
		{
			name:        "from is a 32 byte address",
			fromAddress: interchainAccountStyleAddress,
			expectedErr: sdkerrors.ErrUnauthorized,
		},
	}

	for _, directDispatchCase := range directDispatchCases {
		t.Run("msg handler rejects direct router dispatch when "+directDispatchCase.name, func(t *testing.T) {
			dispatchedMsg := newTransferMsg(directDispatchCase.fromAddress)
			require.NoError(t, dispatchedMsg.ValidateBasic())

			msgRouterHandler := evmdApp.MsgServiceRouter().Handler(dispatchedMsg)
			require.NotNil(t, msgRouterHandler)

			signerBalanceBefore := evmdApp.BankKeeper.GetBalance(unitNetwork.GetContext(), rawSignerAccAddress, evmDenom)

			_, err := msgRouterHandler(unitNetwork.GetContext(), dispatchedMsg)

			require.ErrorIs(t, err, directDispatchCase.expectedErr)

			signerBalanceAfter := evmdApp.BankKeeper.GetBalance(unitNetwork.GetContext(), rawSignerAccAddress, evmDenom)
			require.Equal(t, signerBalanceBefore, signerBalanceAfter, "a rejected message must not refund or charge the raw signer")
		})
	}
}
