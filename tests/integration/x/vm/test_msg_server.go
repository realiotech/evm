package vm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	"github.com/cosmos/evm/testutil/integration/evm/utils"
	"github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

func (s *KeeperTestSuite) TestEthereumTx() {
	s.EnableFeemarket = true
	defer func() { s.EnableFeemarket = false }()
	s.SetupTest()

	args := types.EvmTxArgs{
		// Have insufficient gas
		GasLimit: 10,
	}
	_, err := s.Factory.GenerateSignedEthTx(s.Keyring.GetPrivKey(0), args)
	s.Require().Error(err)

	testCases := []struct {
		name        string
		getMsg      func() *types.MsgEthereumTx
		expectedErr error
		postCheck   func()
	}{
		{
			name: "success - transfer funds tx",
			getMsg: func() *types.MsgEthereumTx {
				recipient := s.Keyring.GetAddr(1)
				args := types.EvmTxArgs{
					To:     &recipient,
					Amount: big.NewInt(1e18),
				}
				tx, err := s.Factory.GenerateSignedEthTx(s.Keyring.GetPrivKey(0), args)
				s.Require().NoError(err)
				return tx.GetMsgs()[0].(*types.MsgEthereumTx)
			},
			expectedErr: nil,
			postCheck:   nil,
		},
		{
			name: "success - set code authorization tx (EIP-7702)",
			getMsg: func() *types.MsgEthereumTx {
				authority := s.Keyring.GetKey(0)
				target := s.Keyring.GetAddr(1)

				accResp, err := s.Handler.GetEvmAccount(authority.Addr)
				s.Require().NoError(err)

				auth := ethtypes.SetCodeAuthorization{
					ChainID: *uint256.NewInt(types.GetChainConfig().GetChainId()),
					Address: target,
					Nonce:   accResp.GetNonce(),
				}
				signedAuth := s.SignSetCodeAuthorization(authority, auth)

				args := types.EvmTxArgs{
					To:                &target,
					AuthorizationList: []ethtypes.SetCodeAuthorization{signedAuth},
				}
				tx, err := s.Factory.GenerateSignedEthTx(s.Keyring.GetPrivKey(0), args)
				s.Require().NoError(err)
				return tx.GetMsgs()[0].(*types.MsgEthereumTx)
			},
			expectedErr: nil,
			postCheck: func() {
				authorityAddr := s.Keyring.GetAddr(0)
				targetAddr := s.Keyring.GetAddr(1)
				codeHash := s.Network.App.GetEVMKeeper().GetCodeHash(s.Network.GetContext(), authorityAddr)
				code := s.Network.App.GetEVMKeeper().GetCode(s.Network.GetContext(), codeHash)
				delegationAddr, ok := ethtypes.ParseDelegation(code)
				s.Require().True(ok)
				s.Require().Equal(targetAddr, delegationAddr)
			},
		},
		{
			name: "fail - unsigned set code authorization",
			getMsg: func() *types.MsgEthereumTx {
				authority := s.Keyring.GetKey(0)
				target := s.Keyring.GetAddr(1)

				accResp, err := s.Handler.GetEvmAccount(authority.Addr)
				s.Require().NoError(err)

				auth := ethtypes.SetCodeAuthorization{
					ChainID: *uint256.NewInt(types.GetChainConfig().GetChainId()),
					Address: target,
					Nonce:   accResp.GetNonce(),
				}

				args := types.EvmTxArgs{
					To:                &target,
					AuthorizationList: []ethtypes.SetCodeAuthorization{auth},
				}
				tx, err := s.Factory.GenerateSignedEthTx(s.Keyring.GetPrivKey(0), args)
				s.Require().NoError(err)
				return tx.GetMsgs()[0].(*types.MsgEthereumTx)
			},
			expectedErr: nil,
			postCheck: func() {
				authorityAddr := s.Keyring.GetAddr(0)
				codeHash := s.Network.App.GetEVMKeeper().GetCodeHash(s.Network.GetContext(), authorityAddr)
				code := s.Network.App.GetEVMKeeper().GetCode(s.Network.GetContext(), codeHash)
				_, ok := ethtypes.ParseDelegation(code)
				s.Require().False(ok)
				s.Require().Len(code, 0)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Fund fee collector account
			ctx := s.Network.GetContext()
			coins := sdktypes.NewCoins(sdktypes.NewCoin(types.GetEVMCoinDenom(), sdkmath.NewInt(1e18)))
			err := s.Network.App.GetBankKeeper().MintCoins(ctx, "mint", coins)
			s.Require().NoError(err)
			err = s.Network.App.GetBankKeeper().SendCoinsFromModuleToModule(ctx, "mint", "fee_collector", coins)
			s.Require().NoError(err)

			// Get EthereumTx msg
			msg := tc.getMsg()

			// Function to be tested
			res, err := utils.ExecuteAnteVerifiedEthereumTx(ctx, s.Network.App.GetEVMKeeper(), msg)

			events := s.Network.GetContext().EventManager().Events()
			if tc.expectedErr != nil {
				s.Require().Error(err)
				// no events should have been emitted
				s.Require().Empty(events)
			} else {
				s.Require().NoError(err)
				s.Require().False(res.Failed())

				// check expected events were emitted
				s.Require().NotEmpty(events)
				s.Require().True(utils.ContainsEventType(events.ToABCIEvents(), types.EventTypeEthereumTx))
				s.Require().True(utils.ContainsEventType(events.ToABCIEvents(), sdktypes.EventTypeMessage))
			}

			if tc.postCheck != nil {
				tc.postCheck()
			}

			err = s.Network.NextBlock()
			s.Require().NoError(err)
		})
	}
	s.EnableFeemarket = false
}

// TestEthereumTxRequiresAnteVerifiedSender checks that the MsgEthereumTx
// handler rejects, without moving funds, a message the EVM ante handler did not
// record for the same sender in the same tx.
func (s *KeeperTestSuite) TestEthereumTxRequiresAnteVerifiedSender() {
	anteCosmosTxBytes := []byte("cosmos tx the ante handler verified")
	laterCosmosTxBytes := []byte("later cosmos tx of the block")

	rawSignerAddress := s.Keyring.GetAddr(0)
	recipientAddress := s.Keyring.GetAddr(1)
	unrelatedAddress := common.HexToAddress("0x00000000000000000000000000000000000BEEF1")

	testCases := []struct {
		name string
		// arrange prepares the context the handler is called with and may
		// change the message it is called with.
		arrange     func(ctx sdktypes.Context, transferMsg *types.MsgEthereumTx) sdktypes.Context
		expectedErr error
	}{
		{
			name: "fail - ante handler did not run for the message",
			arrange: func(ctx sdktypes.Context, _ *types.MsgEthereumTx) sdktypes.Context {
				return ctx
			},
			expectedErr: sdkerrors.ErrUnauthorized,
		},
		{
			name: "fail - from differs from the sender the ante handler verified",
			arrange: func(ctx sdktypes.Context, transferMsg *types.MsgEthereumTx) sdktypes.Context {
				s.Network.App.GetEVMKeeper().SetAnteVerifiedSender(ctx, transferMsg.Hash(), rawSignerAddress)

				transferMsg.From = unrelatedAddress.Bytes()

				return ctx
			},
			expectedErr: sdkerrors.ErrorInvalidSigner,
		},
		{
			name: "fail - from is longer than an ethereum address",
			arrange: func(ctx sdktypes.Context, transferMsg *types.MsgEthereumTx) sdktypes.Context {
				s.Network.App.GetEVMKeeper().SetAnteVerifiedSender(ctx, transferMsg.Hash(), rawSignerAddress)

				transferMsg.From = append(make([]byte, 12), rawSignerAddress.Bytes()...)

				return ctx
			},
			expectedErr: sdkerrors.ErrorInvalidSigner,
		},
		{
			name: "fail - ante handler verified the message in an earlier tx of the block",
			arrange: func(ctx sdktypes.Context, transferMsg *types.MsgEthereumTx) sdktypes.Context {
				s.Network.App.GetEVMKeeper().SetAnteVerifiedSender(ctx, transferMsg.Hash(), rawSignerAddress)

				return ctx.WithTxBytes(laterCosmosTxBytes)
			},
			expectedErr: sdkerrors.ErrUnauthorized,
		},
		{
			name: "fail - message was already executed once",
			arrange: func(ctx sdktypes.Context, transferMsg *types.MsgEthereumTx) sdktypes.Context {
				firstResponse, err := utils.ExecuteAnteVerifiedEthereumTx(ctx, s.Network.App.GetEVMKeeper(), transferMsg)
				s.Require().NoError(err)
				s.Require().False(firstResponse.Failed())

				return ctx
			},
			expectedErr: sdkerrors.ErrUnauthorized,
		},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			s.SetupTest()

			evmKeeper := s.Network.App.GetEVMKeeper()
			bankKeeper := s.Network.App.GetBankKeeper()
			evmDenom := types.GetEVMCoinDenom()

			// The handler refunds unused gas out of the fee collector's balance.
			anteCtx := s.Network.GetContext().WithTxBytes(anteCosmosTxBytes)
			refundableFees := sdktypes.NewCoins(sdktypes.NewCoin(evmDenom, sdkmath.NewInt(1e18)))
			err := bankKeeper.MintCoins(anteCtx, "mint", refundableFees)
			s.Require().NoError(err)

			err = bankKeeper.SendCoinsFromModuleToModule(anteCtx, "mint", authtypes.FeeCollectorName, refundableFees)
			s.Require().NoError(err)

			transferArgs := types.EvmTxArgs{
				To:       &recipientAddress,
				Amount:   big.NewInt(1e18),
				GasLimit: 1_000_000,
			}
			transferMsg, err := s.Factory.GenerateSignedMsgEthereumTx(s.Keyring.GetPrivKey(0), transferArgs)
			s.Require().NoError(err)

			handlerCtx := testCase.arrange(anteCtx, &transferMsg)

			signerBalanceBefore := bankKeeper.GetBalance(handlerCtx, rawSignerAddress.Bytes(), evmDenom)
			recipientBalanceBefore := bankKeeper.GetBalance(handlerCtx, recipientAddress.Bytes(), evmDenom)

			response, err := evmKeeper.EthereumTx(handlerCtx, &transferMsg)

			s.Require().ErrorIs(err, testCase.expectedErr)
			s.Require().Nil(response)

			signerBalanceAfter := bankKeeper.GetBalance(handlerCtx, rawSignerAddress.Bytes(), evmDenom)
			recipientBalanceAfter := bankKeeper.GetBalance(handlerCtx, recipientAddress.Bytes(), evmDenom)
			s.Require().Equal(signerBalanceBefore, signerBalanceAfter, "a rejected message must not refund or charge the raw signer")
			s.Require().Equal(recipientBalanceBefore, recipientBalanceAfter, "a rejected message must not transfer value")
		})
	}
}

func (s *KeeperTestSuite) TestUpdateParams() {
	s.SetupTest()
	testCases := []struct {
		name        string
		getMsg      func() *types.MsgUpdateParams
		expectedErr error
	}{
		{
			name: "fail - invalid authority",
			getMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{Authority: "foobar"}
			},
			expectedErr: govtypes.ErrInvalidSigner,
		},
		{
			name: "pass - valid Update msg",
			getMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: authtypes.NewModuleAddress(govtypes.ModuleName).String(),
					Params:    types.DefaultParams(),
				}
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		s.Run("MsgUpdateParams", func() {
			msg := tc.getMsg()
			_, err := s.Network.App.GetEVMKeeper().UpdateParams(s.Network.GetContext(), msg)
			if tc.expectedErr != nil {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.expectedErr.Error())
			} else {
				s.Require().NoError(err)
			}
		})

		err := s.Network.NextBlock()
		s.Require().NoError(err)
	}
}

func (s *KeeperTestSuite) TestRegisterPreinstalls() {
	s.SetupTest()
	testCases := []struct {
		name        string
		getMsg      func() *types.MsgRegisterPreinstalls
		expectedErr error
	}{
		{
			name: "fail - invalid authority",
			getMsg: func() *types.MsgRegisterPreinstalls {
				return &types.MsgRegisterPreinstalls{Authority: "foobar"}
			},
			expectedErr: govtypes.ErrInvalidSigner,
		},
		{
			name: "pass - valid Update msg",
			getMsg: func() *types.MsgRegisterPreinstalls {
				return &types.MsgRegisterPreinstalls{
					Authority: authtypes.NewModuleAddress(govtypes.ModuleName).String(),
					Preinstalls: []types.Preinstall{{
						Name:    "Test1",
						Address: "0xb364E75b1189DcbBF7f0C856456c1ba8e4d6481b",
						Code:    "0x000000000",
					}},
				}
			},
			expectedErr: nil,
		},
		{
			name: "fail - double registration",
			getMsg: func() *types.MsgRegisterPreinstalls {
				return &types.MsgRegisterPreinstalls{
					Authority:   authtypes.NewModuleAddress(govtypes.ModuleName).String(),
					Preinstalls: types.DefaultPreinstalls,
				}
			},
			expectedErr: types.ErrInvalidPreinstall,
		},
	}

	for _, tc := range testCases {
		s.Run("MsgRegisterPreinstalls_"+tc.name, func() {
			msg := tc.getMsg()
			_, err := s.Network.App.GetEVMKeeper().RegisterPreinstalls(s.Network.GetContext(), msg)
			if tc.expectedErr != nil {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.expectedErr.Error())
			} else {
				s.Require().NoError(err)
			}
		})

		err := s.Network.NextBlock()
		s.Require().NoError(err)
	}
}
