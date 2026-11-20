package erc20

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/stretchr/testify/mock"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/utils"
	"github.com/cosmos/evm/x/erc20/keeper"
	"github.com/cosmos/evm/x/erc20/types"
	erc20mocks "github.com/cosmos/evm/x/erc20/types/mocks"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibcgotesting "github.com/cosmos/ibc-go/v10/testing"
	ibcmock "github.com/cosmos/ibc-go/v10/testing/mock"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

var erc20Denom = "erc20:0xdac17f958d2ee523a2206206994597c13d831ec7"

func (s *KeeperTestSuite) TestOnRecvPacketRegistered() {
	var ctx sdk.Context
	// secp256k1 account
	secpPk := secp256k1.GenPrivKey()
	secpAddr := sdk.AccAddress(secpPk.PubKey().Address())
	secpAddrCosmos := sdk.MustBech32ifyAddressBytes(sdk.Bech32MainPrefix, secpAddr)

	// ethsecp256k1 account
	ethPk, err := ethsecp256k1.GenerateKey()
	s.Require().Nil(err)
	ethsecpAddr := sdk.AccAddress(ethPk.PubKey().Address())
	ethsecpAddrEvmos := sdk.AccAddress(ethPk.PubKey().Address()).String()
	ethsecpAddrCosmos := sdk.MustBech32ifyAddressBytes(sdk.Bech32MainPrefix, ethsecpAddr)

	// Setup Cosmos <=> Cosmos EVM IBC relayer
	sourceChannel := "channel-292"
	cosmosEVMChannel := "channel-3"
	hop := transfertypes.NewHop(transfertypes.PortID, cosmosEVMChannel)

	timeoutHeight := clienttypes.NewHeight(0, 100)
	disabledTimeoutTimestamp := uint64(0)
	mockPacket := channeltypes.NewPacket(ibcgotesting.MockPacketData, 1, transfertypes.PortID, "channel-0", transfertypes.PortID, "channel-0", timeoutHeight, disabledTimeoutTimestamp)
	packet := mockPacket
	expAck := ibcmock.MockAcknowledgement

	baseDenom, err := sdk.GetBaseDenom()
	s.Require().NoError(err, "failed to get base denom")
	registeredDenom := cosmosTokenBase
	coins := sdk.NewCoins(
		sdk.NewCoin(baseDenom, math.NewInt(1000)),
		sdk.NewCoin(registeredDenom, math.NewInt(1000)), // some ERC20 token
		sdk.NewCoin(ibcBase, math.NewInt(1000)),         // some IBC coin with a registered token pair
	)

	testCases := []struct {
		name             string
		malleate         func()
		ackSuccess       bool
		receiver         sdk.AccAddress
		expErc20s        *big.Int
		expCoins         sdk.Coins
		checkBalances    bool
		disableERC20     bool
		disableTokenPair bool
	}{
		{
			name: "error - non ics-20 packet",
			malleate: func() {
				packet = mockPacket
			},
			receiver:      secpAddr,
			ackSuccess:    false,
			checkBalances: false,
			expErc20s:     big.NewInt(0),
			expCoins:      coins,
		},
		{
			name: "no-op - erc20 module param disabled",
			malleate: func() {
				transfer := transfertypes.NewFungibleTokenPacketData(registeredDenom, "100", ethsecpAddrEvmos, ethsecpAddrCosmos, "")
				bz := transfertypes.ModuleCdc.MustMarshalJSON(&transfer)
				packet = channeltypes.NewPacket(bz, 1, transfertypes.PortID, sourceChannel, transfertypes.PortID, cosmosEVMChannel, timeoutHeight, 0)
			},
			receiver:      secpAddr,
			disableERC20:  true,
			ackSuccess:    true,
			checkBalances: false,
			expErc20s:     big.NewInt(0),
			expCoins:      coins,
		},
		{
			name: "success - invalid sender (no '1')",
			malleate: func() {
				transfer := transfertypes.NewFungibleTokenPacketData(registeredDenom, "100", "evmos", ethsecpAddrCosmos, "")
				bz := transfertypes.ModuleCdc.MustMarshalJSON(&transfer)
				packet = channeltypes.NewPacket(bz, 100, transfertypes.PortID, sourceChannel, transfertypes.PortID, cosmosEVMChannel, timeoutHeight, 0)
			},
			receiver:      secpAddr,
			ackSuccess:    true,
			checkBalances: true,
			expErc20s:     big.NewInt(0),
			expCoins:      coins,
		},
		{
			name: "success - invalid sender (bad address)",
			malleate: func() {
				transfer := transfertypes.NewFungibleTokenPacketData(registeredDenom, "100", "badba1sv9m0g7ycejwr3s369km58h5qe7xj77hvcxrms", ethsecpAddrCosmos, "")
				bz := transfertypes.ModuleCdc.MustMarshalJSON(&transfer)
				packet = channeltypes.NewPacket(bz, 100, transfertypes.PortID, sourceChannel, transfertypes.PortID, cosmosEVMChannel, timeoutHeight, 0)
			},
			receiver:      secpAddr,
			ackSuccess:    true,
			checkBalances: true,
			expErc20s:     big.NewInt(0),
			expCoins:      coins,
		},
		{
			name: "error - invalid recipient (bad address)",
			malleate: func() {
				transfer := transfertypes.NewFungibleTokenPacketData(registeredDenom, "100", ethsecpAddrEvmos, "badbadhf0468jjpe6m6vx38s97z2qqe8ldu0njdyf625", "")
				bz := transfertypes.ModuleCdc.MustMarshalJSON(&transfer)
				packet = channeltypes.NewPacket(bz, 100, transfertypes.PortID, sourceChannel, transfertypes.PortID, cosmosEVMChannel, timeoutHeight, 0)
			},
			receiver:      secpAddr,
			ackSuccess:    false,
			checkBalances: false,
			expErc20s:     big.NewInt(0),
			expCoins:      coins,
		},
		{
			name: "no-op - receiver is module account",
			malleate: func() {
				secpAddr = s.network.App.GetAccountKeeper().GetModuleAccount(ctx, "erc20").GetAddress()
				transfer := transfertypes.NewFungibleTokenPacketData(registeredDenom, "100", secpAddrCosmos, secpAddr.String(), "")
				bz := transfertypes.ModuleCdc.MustMarshalJSON(&transfer)
				packet = channeltypes.NewPacket(bz, 100, transfertypes.PortID, sourceChannel, transfertypes.PortID, cosmosEVMChannel, timeoutHeight, 0)
			},
			ackSuccess:    true,
			receiver:      secpAddr,
			expErc20s:     big.NewInt(0),
			expCoins:      coins,
			checkBalances: true,
		},
		{
			name: "no-op - base denomination",
			malleate: func() {
				// base denom should be prefixed
				hop := transfertypes.NewHop(transfertypes.PortID, sourceChannel)
				bondDenom, err := s.network.App.GetStakingKeeper().BondDenom(ctx)
				s.Require().NoError(err)
				prefixedDenom := transfertypes.NewDenom(bondDenom, hop).Path()
				transfer := transfertypes.NewFungibleTokenPacketData(prefixedDenom, "100", secpAddrCosmos, ethsecpAddrEvmos, "")
				bz := transfertypes.ModuleCdc.MustMarshalJSON(&transfer)
				packet = channeltypes.NewPacket(bz, 1, transfertypes.PortID, sourceChannel, transfertypes.PortID, cosmosEVMChannel, timeoutHeight, 0)
			},
			ackSuccess:    true,
			receiver:      ethsecpAddr,
			expErc20s:     big.NewInt(0),
			expCoins:      coins,
			checkBalances: true,
		},
		{
			name: "no-op - pair is not registered",
			malleate: func() {
				transfer := transfertypes.NewFungibleTokenPacketData(erc20Denom, "100", secpAddrCosmos, ethsecpAddrEvmos, "")
				bz := transfertypes.ModuleCdc.MustMarshalJSON(&transfer)
				packet = channeltypes.NewPacket(bz, 1, transfertypes.PortID, sourceChannel, transfertypes.PortID, cosmosEVMChannel, timeoutHeight, 0)
			},
			ackSuccess:    true,
			receiver:      ethsecpAddr,
			expErc20s:     big.NewInt(0),
			expCoins:      coins,
			checkBalances: true,
		},
		{
			name: "error - pair is not registered but erc20 registered",
			malleate: func() {
				transfer := transfertypes.NewFungibleTokenPacketData(erc20Denom, "100", secpAddrCosmos, ethsecpAddrEvmos, "")
				bz := transfertypes.ModuleCdc.MustMarshalJSON(&transfer)
				packet = channeltypes.NewPacket(bz, 1, transfertypes.PortID, sourceChannel, transfertypes.PortID, cosmosEVMChannel, timeoutHeight, 0)
				collidedAddr, err := utils.GetIBCDenomAddress(transfertypes.NewDenom(erc20Denom, hop).IBCDenom())
				s.Require().NoError(err)
				s.network.App.GetErc20Keeper().SetERC20Map(ctx, collidedAddr, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
				s.Require().True(s.network.App.GetErc20Keeper().IsERC20Registered(ctx, collidedAddr))
			},
			ackSuccess:    false,
			receiver:      secpAddr,
			expErc20s:     big.NewInt(0),
			expCoins:      coins,
			checkBalances: false,
		},
		{
			name: "error - pair is not registered but denom registered",
			malleate: func() {
				transfer := transfertypes.NewFungibleTokenPacketData(erc20Denom, "100", secpAddrCosmos, ethsecpAddrEvmos, "")
				bz := transfertypes.ModuleCdc.MustMarshalJSON(&transfer)
				packet = channeltypes.NewPacket(bz, 1, transfertypes.PortID, sourceChannel, transfertypes.PortID, cosmosEVMChannel, timeoutHeight, 0)
				collidedDenom := transfertypes.NewDenom(erc20Denom, hop).IBCDenom()
				s.network.App.GetErc20Keeper().SetDenomMap(ctx, collidedDenom, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
				s.Require().True(s.network.App.GetErc20Keeper().IsDenomRegistered(ctx, collidedDenom))
			},
			ackSuccess:    false,
			receiver:      secpAddr,
			expErc20s:     big.NewInt(0),
			expCoins:      coins,
			checkBalances: false,
		},
		{
			name: "error - pair is not registered but address has code",
			malleate: func() {
				transfer := transfertypes.NewFungibleTokenPacketData(erc20Denom, "100", secpAddrCosmos, ethsecpAddrEvmos, "")
				bz := transfertypes.ModuleCdc.MustMarshalJSON(&transfer)
				packet = channeltypes.NewPacket(bz, 1, transfertypes.PortID, sourceChannel, transfertypes.PortID, cosmosEVMChannel, timeoutHeight, 0)
				collidedAddr, err := utils.GetIBCDenomAddress(transfertypes.NewDenom(erc20Denom, hop).IBCDenom())
				s.Require().NoError(err)
				s.Require().False(s.network.App.GetErc20Keeper().IsERC20Registered(ctx, collidedAddr))
				err = s.network.App.GetEVMKeeper().SetAccount(ctx, collidedAddr, statedb.Account{
					Nonce:    0,
					Balance:  nil,
					CodeHash: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
				})
				s.Require().NoError(err)
				acct := s.network.App.GetEVMKeeper().GetAccount(ctx, collidedAddr)
				s.Require().True(acct.HasCodeHash())
			},
			ackSuccess:    false,
			receiver:      secpAddr,
			expErc20s:     big.NewInt(0),
			expCoins:      coins,
			checkBalances: false,
		},
		{
			name: "no-op - pair disabled",
			malleate: func() {
				pk1 := secp256k1.GenPrivKey()
				hop := transfertypes.NewHop(transfertypes.PortID, sourceChannel)
				prefixedDenom := transfertypes.NewDenom(registeredDenom, hop).Path()
				otherSecpAddrEvmos := sdk.AccAddress(pk1.PubKey().Address()).String()
				transfer := transfertypes.NewFungibleTokenPacketData(prefixedDenom, "500", otherSecpAddrEvmos, ethsecpAddrEvmos, "")
				bz := transfertypes.ModuleCdc.MustMarshalJSON(&transfer)
				packet = channeltypes.NewPacket(bz, 1, transfertypes.PortID, sourceChannel, transfertypes.PortID, cosmosEVMChannel, timeoutHeight, 0)
			},
			ackSuccess: true,
			receiver:   ethsecpAddr,
			expErc20s:  big.NewInt(0),
			expCoins: sdk.NewCoins(
				sdk.NewCoin(baseDenom, math.NewInt(1000)),
				sdk.NewCoin(registeredDenom, math.NewInt(0)),
				sdk.NewCoin(ibcBase, math.NewInt(1000)),
			),
			checkBalances:    false,
			disableTokenPair: true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.mintFeeCollector = true
			s.SetupTest() // reset
			ctx = s.network.GetContext()

			// Register Token Pair for testing
			contractAddr, err := s.setupRegisterERC20Pair(contractMinterBurner)
			s.Require().NoError(err, "failed to register pair")
			// get updated context after registering ERC20 pair
			ctx = s.network.GetContext()

			// Set Denom
			denom := transfertypes.NewDenom(registeredDenom, hop)
			s.network.App.GetTransferKeeper().SetDenom(ctx, denom)

			// Set Cosmos Channel
			channel := channeltypes.Channel{
				State:          channeltypes.INIT,
				Ordering:       channeltypes.UNORDERED,
				Counterparty:   channeltypes.NewCounterparty(transfertypes.PortID, sourceChannel),
				ConnectionHops: []string{sourceChannel},
			}
			s.network.App.GetIBCKeeper().ChannelKeeper.SetChannel(ctx, transfertypes.PortID, cosmosEVMChannel, channel)

			// Set Next Sequence Send
			s.network.App.GetIBCKeeper().ChannelKeeper.SetNextSequenceSend(ctx, transfertypes.PortID, cosmosEVMChannel, 1)

			tranasferKeeper := s.network.App.GetTransferKeeper()
			erc20Keeper := keeper.NewKeeper(
				s.network.App.GetKey(types.StoreKey),
				s.network.App.AppCodec(),
				authtypes.NewModuleAddress(govtypes.ModuleName),
				s.network.App.GetAccountKeeper(),
				s.network.App.GetBankKeeper(),
				s.network.App.GetEVMKeeper(),
				s.network.App.GetStakingKeeper(),
				&tranasferKeeper,
			)
			s.network.App.SetErc20Keeper(erc20Keeper)

			// Fund receiver account with ATOM, ERC20 coins and IBC vouchers
			// We do this since we are interested in the conversion portion w/ OnRecvPacket
			err = testutil.FundAccount(ctx, s.network.App.GetBankKeeper(), tc.receiver, coins)
			s.Require().NoError(err)

			id := s.network.App.GetErc20Keeper().GetTokenPairID(ctx, contractAddr.String())
			pair, _ := s.network.App.GetErc20Keeper().GetTokenPair(ctx, id)
			s.Require().NotNil(pair)

			if tc.disableERC20 {
				params := s.network.App.GetErc20Keeper().GetParams(ctx)
				params.EnableErc20 = false
				s.network.App.GetErc20Keeper().SetParams(ctx, params) //nolint:errcheck
			}

			if tc.disableTokenPair {
				_, err := s.network.App.GetErc20Keeper().ToggleConversion(ctx, &types.MsgToggleConversion{
					Authority: authtypes.NewModuleAddress("gov").String(),
					Token:     pair.Denom,
				})
				s.Require().NoError(err)
			}

			tc.malleate()

			// Perform IBC callback
			ack := s.network.App.GetErc20Keeper().OnRecvPacket(ctx, packet, expAck)

			// Check acknowledgement
			if tc.ackSuccess {
				s.Require().True(ack.Success(), string(ack.Acknowledgement()))
				s.Require().Equal(expAck, ack)
			} else {
				s.Require().False(ack.Success(), string(ack.Acknowledgement()))
			}

			if tc.checkBalances {
				// Check ERC20 balances
				balanceTokenAfter := s.network.App.GetErc20Keeper().BalanceOf(ctx, contracts.ERC20MinterBurnerDecimalsContract.ABI, pair.GetERC20Contract(), common.BytesToAddress(tc.receiver.Bytes()))
				s.Require().Equal(tc.expErc20s.Int64(), balanceTokenAfter.Int64())
				// Check Cosmos Coin Balances
				balances := s.network.App.GetBankKeeper().GetAllBalances(ctx, tc.receiver)
				s.Require().Equal(tc.expCoins, balances)
			}
		})
	}
}

func (s *KeeperTestSuite) TestConvertCoinToERC20FromPacket() {
	var ctx sdk.Context
	senderAddr := "cosmos1x2w87cvt5mqjncav4lxy8yfreynn273x34qlwy"

	baseDenom, err := sdk.GetBaseDenom()
	s.Require().NoError(err)

	testCases := []struct {
		name     string
		malleate func() transfertypes.FungibleTokenPacketData
		transfer transfertypes.FungibleTokenPacketData
		expPass  bool
	}{
		{
			name: "error - invalid sender",
			malleate: func() transfertypes.FungibleTokenPacketData {
				return transfertypes.NewFungibleTokenPacketData(baseDenom, "10", "", "", "")
			},
			expPass: false,
		},
		{
			name: "pass - is base denom",
			malleate: func() transfertypes.FungibleTokenPacketData {
				return transfertypes.NewFungibleTokenPacketData(baseDenom, "10", senderAddr, "", "")
			},
			expPass: true,
		},
		{
			name: "pass - erc20 is disabled",
			malleate: func() transfertypes.FungibleTokenPacketData {
				// Register Token Pair for testing
				contractAddr, err := s.setupRegisterERC20Pair(contractMinterBurner)
				s.Require().NoError(err, "failed to register pair")
				ctx = s.network.GetContext()
				id := s.network.App.GetErc20Keeper().GetTokenPairID(ctx, contractAddr.String())
				pair, _ := s.network.App.GetErc20Keeper().GetTokenPair(ctx, id)
				s.Require().NotNil(pair)

				params := s.network.App.GetErc20Keeper().GetParams(ctx)
				params.EnableErc20 = false
				_ = s.network.App.GetErc20Keeper().SetParams(ctx, params)
				return transfertypes.NewFungibleTokenPacketData(pair.Denom, "10", senderAddr, "", "")
			},
			expPass: true,
		},
		{
			name: "pass - denom is not registered",
			malleate: func() transfertypes.FungibleTokenPacketData {
				return transfertypes.NewFungibleTokenPacketData(metadataIbc.Base, "10", senderAddr, "", "")
			},
			expPass: true,
		},
		{
			name: "pass - erc20 is disabled",
			malleate: func() transfertypes.FungibleTokenPacketData {
				// Register Token Pair for testing
				contractAddr, err := s.setupRegisterERC20Pair(contractMinterBurner)
				s.Require().NoError(err, "failed to register pair")
				ctx = s.network.GetContext()
				id := s.network.App.GetErc20Keeper().GetTokenPairID(ctx, contractAddr.String())
				pair, _ := s.network.App.GetErc20Keeper().GetTokenPair(ctx, id)
				s.Require().NotNil(pair)

				err = testutil.FundAccount(
					ctx,
					s.network.App.GetBankKeeper(),
					sdk.MustAccAddressFromBech32(senderAddr),
					sdk.NewCoins(
						sdk.NewCoin(pair.Denom, math.NewInt(100)),
					),
				)
				s.Require().NoError(err)
				stateDB := statedb.New(ctx, s.network.App.GetEVMKeeper(), statedb.NewEmptyTxConfig())
				_, err = s.network.App.GetEVMKeeper().CallEVM(ctx, stateDB, contracts.ERC20MinterBurnerDecimalsContract.ABI, s.keyring.GetAddr(0), contractAddr, true, false, nil, "mint", types.ModuleAddress, big.NewInt(10))
				s.Require().NoError(err)

				return transfertypes.NewFungibleTokenPacketData(pair.Denom, "10", senderAddr, "", "")
			},
			expPass: true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.mintFeeCollector = true
			defer func() { s.mintFeeCollector = false }()

			s.SetupTest() // reset
			ctx = s.network.GetContext()

			transfer := tc.malleate()

			err := s.network.App.GetErc20Keeper().ConvertCoinToERC20FromPacket(ctx, transfer)
			if tc.expPass {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

// honestTokenRefund is a refund of a well-behaved registered ERC20's coins that is
// waiting to be reconverted: the refunded account holds the bank coins and the erc20
// module holds the tokens to hand back.
type honestTokenRefund struct {
	ctx             sdk.Context
	contractAddress common.Address
	tokenPair       types.TokenPair
	refundedAccount sdk.AccAddress
	refundAmount    math.Int
	packetData      transfertypes.FungibleTokenPacketData
}

// setupHonestTokenRefund registers a well-behaved ERC20 and returns a refund of its
// coins that ConvertCoinToERC20FromPacket can reconvert.
func (s *KeeperTestSuite) setupHonestTokenRefund() honestTokenRefund {
	contractAddress, err := s.setupRegisterERC20Pair(contractMinterBurner)
	s.Require().NoError(err)

	ctx := s.network.GetContext()
	tokenPairID := s.network.App.GetErc20Keeper().GetTokenPairID(ctx, contractAddress.String())
	tokenPair, found := s.network.App.GetErc20Keeper().GetTokenPair(ctx, tokenPairID)
	s.Require().True(found)

	refundedAccount := sdk.AccAddress(s.keyring.GetAddr(0).Bytes())
	refundAmount := math.NewInt(100)
	refundedCoins := sdk.NewCoins(sdk.NewCoin(tokenPair.Denom, refundAmount))
	s.Require().NoError(testutil.FundAccount(ctx, s.network.App.GetBankKeeper(), refundedAccount, refundedCoins))

	stateDB := statedb.New(ctx, s.network.App.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	_, err = s.network.App.GetEVMKeeper().CallEVM(
		ctx, stateDB, contracts.ERC20MinterBurnerDecimalsContract.ABI, s.keyring.GetAddr(0), contractAddress,
		true, false, nil, "mint", types.ModuleAddress, refundAmount.BigInt(),
	)
	s.Require().NoError(err)

	return honestTokenRefund{
		ctx:             ctx,
		contractAddress: contractAddress,
		tokenPair:       tokenPair,
		refundedAccount: refundedAccount,
		refundAmount:    refundAmount,
		packetData:      transfertypes.NewFungibleTokenPacketData(tokenPair.Denom, refundAmount.String(), refundedAccount.String(), refundedAccount.String(), ""),
	}
}

// simulateRefundConversionGas returns the gas ConvertCoinToERC20FromPacket uses for
// the refund, measured as a relayer's simulation does: on an infinite gas meter and
// without committing state.
func (s *KeeperTestSuite) simulateRefundConversionGas(pendingRefund honestTokenRefund) uint64 {
	simulationGasMeter := storetypes.NewInfiniteGasMeter()
	simulationCtx, _ := pendingRefund.ctx.WithGasMeter(simulationGasMeter).CacheContext()

	err := s.network.App.GetErc20Keeper().ConvertCoinToERC20FromPacket(simulationCtx, pendingRefund.packetData)
	s.Require().NoError(err)

	simulatedGas := simulationGasMeter.GasConsumed()
	s.Require().Positive(simulatedGas)

	return simulatedGas
}

// requireRefundHeldAs asserts how much of the refund the refunded account holds as
// ERC20 tokens and how much as bank coins.
func (s *KeeperTestSuite) requireRefundHeldAs(pendingRefund honestTokenRefund, expectedTokens, expectedCoins math.Int) {
	senderTokens := s.network.App.GetErc20Keeper().BalanceOf(
		pendingRefund.ctx, contracts.ERC20MinterBurnerDecimalsContract.ABI,
		pendingRefund.contractAddress, common.BytesToAddress(pendingRefund.refundedAccount),
	)
	s.Require().Equal(expectedTokens.String(), senderTokens.String())

	senderCoins := s.network.App.GetBankKeeper().GetBalance(pendingRefund.ctx, pendingRefund.refundedAccount, pendingRefund.tokenPair.Denom)
	s.Require().Equal(expectedCoins.String(), senderCoins.Amount.String())
}

func (s *KeeperTestSuite) TestConvertCoinToERC20FromPacketReconvertsAtSimulatedGasPlusTenPercent() {
	s.mintFeeCollector = true
	defer func() { s.mintFeeCollector = false }()
	s.SetupTest()

	pendingRefund := s.setupHonestTokenRefund()
	simulatedGas := s.simulateRefundConversionGas(pendingRefund)

	deliveryGasMeter := storetypes.NewGasMeter(simulatedGas + simulatedGas/10)
	deliveryCtx := pendingRefund.ctx.WithGasMeter(deliveryGasMeter)
	s.Require().NoError(s.network.App.GetErc20Keeper().ConvertCoinToERC20FromPacket(deliveryCtx, pendingRefund.packetData))

	s.requireRefundHeldAs(pendingRefund, pendingRefund.refundAmount, math.ZeroInt())
}

func (s *KeeperTestSuite) TestConvertCoinToERC20FromPacketFailsOutOfGasBelowSimulatedGas() {
	s.mintFeeCollector = true
	defer func() { s.mintFeeCollector = false }()
	s.SetupTest()

	pendingRefund := s.setupHonestTokenRefund()
	simulatedGas := s.simulateRefundConversionGas(pendingRefund)

	// One unit short of the simulated gas, the transaction must fail so the relayer
	// can retry, not complete with the refund left as bank coins.
	underfundedCtx := pendingRefund.ctx.WithGasMeter(storetypes.NewGasMeter(simulatedGas - 1))
	s.Require().PanicsWithValue(
		storetypes.ErrorOutOfGas{Descriptor: "convert coin to erc20 from packet"},
		func() {
			_ = s.network.App.GetErc20Keeper().ConvertCoinToERC20FromPacket(underfundedCtx, pendingRefund.packetData)
		},
	)

	s.requireRefundHeldAs(pendingRefund, math.ZeroInt(), pendingRefund.refundAmount)
}

func (s *KeeperTestSuite) TestConvertCoinToERC20FromPacketFailsOutOfGasWhenFirstEVMCallIsRefused() {
	s.mintFeeCollector = true
	defer func() { s.mintFeeCollector = false }()
	s.SetupTest()

	pendingRefund := s.setupHonestTokenRefund()
	tokenABI := contracts.ERC20MinterBurnerDecimalsContract.ABI
	refundedAddress := common.BytesToAddress(pendingRefund.refundedAccount)

	balanceOfCalldata, err := tokenABI.Pack("balanceOf", refundedAddress)
	s.Require().NoError(err)

	intrinsicGas, err := core.IntrinsicGas(balanceOfCalldata, nil, nil, false, true, true, true)
	s.Require().NoError(err)

	floorDataGas, err := core.FloorDataGas(balanceOfCalldata)
	s.Require().NoError(err)

	// A budget just under the floor data gas must still cover the intrinsic gas.
	s.Require().Greater(floorDataGas, intrinsicGas)

	// The EVM refuses a call whose gas limit is below its intrinsic gas, or covers
	// that but not its floor data gas, before any gas is used. Either refusal on an
	// underfunded transaction must fail it, not record a failed conversion.
	refusedCalls := []struct {
		name      string
		gasBudget uint64
		refusal   error
	}{
		{name: "below intrinsic gas", gasBudget: intrinsicGas - 1, refusal: core.ErrIntrinsicGas},
		{name: "below floor data gas", gasBudget: floorDataGas - 1, refusal: core.ErrFloorDataGas},
	}

	// Store charges are switched off so the first balanceOf gets the whole budget.
	freeStoreGasCtx := pendingRefund.ctx.
		WithKVGasConfig(storetypes.GasConfig{}).
		WithTransientKVGasConfig(storetypes.GasConfig{})

	for _, refusedCall := range refusedCalls {
		// The budget really makes the EVM refuse balanceOf for the expected reason.
		stateDB := statedb.New(pendingRefund.ctx, s.network.App.GetEVMKeeper(), statedb.NewEmptyTxConfig())
		directCallCtx := freeStoreGasCtx.WithGasMeter(storetypes.NewGasMeter(refusedCall.gasBudget))
		_, err = s.network.App.GetEVMKeeper().CallEVM(
			directCallCtx, stateDB, tokenABI, types.ModuleAddress, pendingRefund.contractAddress,
			false, false, nil, "balanceOf", refundedAddress,
		)
		s.Require().ErrorIs(err, refusedCall.refusal, refusedCall.name)

		callbackGasMeter := storetypes.NewGasMeter(refusedCall.gasBudget)
		callbackCtx := freeStoreGasCtx.WithGasMeter(callbackGasMeter)
		s.Require().PanicsWithValue(
			storetypes.ErrorOutOfGas{Descriptor: "convert coin to erc20 from packet"},
			func() {
				_ = s.network.App.GetErc20Keeper().ConvertCoinToERC20FromPacket(callbackCtx, pendingRefund.packetData)
			},
			refusedCall.name,
		)

		// The refusal uses no gas, so the callback handed balanceOf the whole budget.
		s.Require().Zero(callbackGasMeter.GasConsumed(), refusedCall.name)
		s.requireRefundHeldAs(pendingRefund, math.ZeroInt(), pendingRefund.refundAmount)
	}
}

func (s *KeeperTestSuite) TestConvertCoinToERC20FromPacketFailedConversionKeepsRefund() {
	s.mintFeeCollector = true
	defer func() { s.mintFeeCollector = false }()
	s.SetupTest()

	contractAddr, err := s.setupRegisterERC20Pair(contractMinterBurner)
	s.Require().NoError(err)

	ctx := s.network.GetContext()
	id := s.network.App.GetErc20Keeper().GetTokenPairID(ctx, contractAddr.String())
	pair, found := s.network.App.GetErc20Keeper().GetTokenPair(ctx, id)
	s.Require().True(found)

	transferKeeper := s.network.App.GetTransferKeeper()
	evmMock := new(erc20mocks.EVMKeeper)
	customKeeper := keeper.NewKeeper(
		s.network.App.GetKey(types.StoreKey),
		s.network.App.AppCodec(),
		authtypes.NewModuleAddress(govtypes.ModuleName),
		s.network.App.GetAccountKeeper(),
		s.network.App.GetBankKeeper(),
		evmMock,
		s.network.App.GetStakingKeeper(),
		&transferKeeper,
	)

	balanceRet, err := contracts.ERC20MinterBurnerDecimalsContract.ABI.Methods["balanceOf"].Outputs.Pack(big.NewInt(0))
	s.Require().NoError(err)
	// balanceOf succeeds, transfer fails after the coins have been escrowed.
	evmMock.On("CallEVM",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, "balanceOf", mock.Anything,
	).Return(&evmtypes.MsgEthereumTxResponse{Ret: balanceRet}, nil)
	var activeParentGasMeter storetypes.GasMeter
	var parentGasBeforeConversion uint64
	evmMock.On("CallEVM",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, "transfer", mock.Anything, mock.Anything,
	).Run(func(args mock.Arguments) {
		parentGasBeforeConversion = activeParentGasMeter.GasConsumed()
		// a reverted EVM call consumes the whole gas meter
		evmCtx := args.Get(0).(sdk.Context)
		evmCtx.GasMeter().ConsumeGas(evmCtx.GasMeter().GasRemaining(), "mock evm revert")
	}).Return((*evmtypes.MsgEthereumTxResponse)(nil), errors.New("mock evm error"))
	evmMock.On("KVStoreKeys").Return(map[string]storetypes.StoreKey{}).Maybe()

	sender := sdk.AccAddress(s.keyring.GetAddr(0).Bytes())
	moduleAddr := authtypes.NewModuleAddress(types.ModuleName)
	bankKeeper := s.network.App.GetBankKeeper()
	amount := math.NewInt(100)
	s.Require().NoError(testutil.FundAccount(ctx, bankKeeper, sender, sdk.NewCoins(sdk.NewCoin(pair.Denom, amount))))

	data := transfertypes.NewFungibleTokenPacketData(pair.Denom, amount.String(), sender.String(), sender.String(), "")
	// A failed conversion must cost a bounded amount whether the relayer sends a large gas
	// limit (delivery) or runs on an infinite meter (simulation, which sets its gas estimate).
	parentGasMetersByName := map[string]storetypes.GasMeter{
		"delivery with a large gas limit": storetypes.NewGasMeter(10_000_000),
		"simulation":                      storetypes.NewInfiniteGasMeter(),
	}
	for meterName, parentGasMeter := range parentGasMetersByName {
		meterCtx := ctx.WithGasMeter(parentGasMeter)
		activeParentGasMeter = parentGasMeter
		s.Require().NoError(customKeeper.ConvertCoinToERC20FromPacket(meterCtx, data), meterName)

		failedConversionGas := parentGasMeter.GasConsumed() - parentGasBeforeConversion
		s.Require().Equal(uint64(1_000_000), failedConversionGas, "%s: failed conversion must charge the reconversion gas cap to the parent", meterName)
		s.Require().Equal(amount, bankKeeper.GetBalance(meterCtx, sender, pair.Denom).Amount, "%s: sender must keep the refunded coins", meterName)
		s.Require().True(bankKeeper.GetBalance(meterCtx, moduleAddr, pair.Denom).Amount.IsZero(), "%s: module must not escrow coins on failed conversion", meterName)
	}
	// the failed transfer must have been attempted
	evmMock.AssertExpectations(s.T())
}

func (s *KeeperTestSuite) TestOnAcknowledgementPacket() {
	var (
		ctx  sdk.Context
		data transfertypes.FungibleTokenPacketData
		ack  channeltypes.Acknowledgement
		pair types.TokenPair
	)

	// secp256k1 account
	senderPk := secp256k1.GenPrivKey()
	sender := sdk.AccAddress(senderPk.PubKey().Address())

	receiverPk := secp256k1.GenPrivKey()
	receiver := sdk.AccAddress(receiverPk.PubKey().Address())
	testCases := []struct {
		name           string
		malleate       func()
		expERC20       *big.Int
		expPass        bool
		expErrorEvents func()
	}{
		{
			name: "no-op - ack error sender is module account",
			malleate: func() {
				// Register Token Pair for testing
				contractAddr, err := s.setupRegisterERC20Pair(contractMinterBurner)
				s.Require().NoError(err, "failed to register pair")
				ctx = s.network.GetContext()
				id := s.network.App.GetErc20Keeper().GetTokenPairID(ctx, contractAddr.String())
				pair, _ = s.network.App.GetErc20Keeper().GetTokenPair(ctx, id)
				s.Require().NotNil(pair)

				// for testing purposes we can only fund is not allowed to receive funds
				moduleAcc := s.network.App.GetAccountKeeper().GetModuleAccount(ctx, "erc20")
				sender = moduleAcc.GetAddress()
				err = testutil.FundModuleAccount(
					ctx,
					s.network.App.GetBankKeeper(),
					moduleAcc.GetName(),
					sdk.NewCoins(
						sdk.NewCoin(pair.Denom, math.NewInt(100)),
					),
				)
				s.Require().NoError(err)

				ack = channeltypes.NewErrorAcknowledgement(errors.New(""))
				data = transfertypes.NewFungibleTokenPacketData(pair.Denom, "100", sender.String(), receiver.String(), "")
			},
			expPass:        true,
			expERC20:       big.NewInt(0),
			expErrorEvents: func() {},
		},
		{
			name: "no-op - positive ack",
			malleate: func() {
				// Register Token Pair for testing
				contractAddr, err := s.setupRegisterERC20Pair(contractMinterBurner)
				s.Require().NoError(err, "failed to register pair")
				ctx = s.network.GetContext()
				id := s.network.App.GetErc20Keeper().GetTokenPairID(ctx, contractAddr.String())
				pair, _ = s.network.App.GetErc20Keeper().GetTokenPair(ctx, id)
				s.Require().NotNil(pair)

				sender = sdk.AccAddress(senderPk.PubKey().Address())

				// Fund receiver account with ATOM, ERC20 coins and IBC vouchers
				// We do this since we are interested in the conversion portion w/ OnRecvPacket
				err = testutil.FundAccount(
					ctx,
					s.network.App.GetBankKeeper(),
					sender,
					sdk.NewCoins(
						sdk.NewCoin(pair.Denom, math.NewInt(100)),
					),
				)
				s.Require().NoError(err)

				ack = channeltypes.NewResultAcknowledgement([]byte{1})
			},
			expERC20:       big.NewInt(0),
			expPass:        true,
			expErrorEvents: func() {},
		},
		{
			name: "convert - error ack",
			malleate: func() {
				// Register Token Pair for testing
				contractAddr, err := s.setupRegisterERC20Pair(contractMinterBurner)
				s.Require().NoError(err, "failed to register pair")
				ctx = s.network.GetContext()
				id := s.network.App.GetErc20Keeper().GetTokenPairID(ctx, contractAddr.String())
				pair, _ = s.network.App.GetErc20Keeper().GetTokenPair(ctx, id)
				s.Require().NotNil(pair)

				sender = sdk.AccAddress(senderPk.PubKey().Address())

				// Fund receiver account with ATOM, ERC20 coins and IBC vouchers
				// We do this since we are interested in the conversion portion w/ OnRecvPacket
				err = testutil.FundAccount(
					ctx,
					s.network.App.GetBankKeeper(),
					sender,
					sdk.NewCoins(
						sdk.NewCoin(pair.Denom, math.NewInt(100)),
					),
				)
				s.Require().NoError(err)

				stateDB := statedb.New(ctx, s.network.App.GetEVMKeeper(), statedb.NewEmptyTxConfig())
				_, err = s.network.App.GetEVMKeeper().CallEVM(ctx, stateDB, contracts.ERC20MinterBurnerDecimalsContract.ABI, s.keyring.GetAddr(0), contractAddr, true, false, nil, "mint", types.ModuleAddress, big.NewInt(100))
				s.Require().NoError(err)

				ack = channeltypes.NewErrorAcknowledgement(errors.New("error"))
				data = transfertypes.NewFungibleTokenPacketData(pair.Denom, "100", sender.String(), receiver.String(), "")
			},
			expERC20:       big.NewInt(100),
			expPass:        true,
			expErrorEvents: func() {},
		},
		{
			name: "err - self-destructed contract",
			malleate: func() {
				// Register Token Pair for testing
				contractAddr, err := s.setupRegisterERC20Pair(contractMinterBurner)
				s.Require().NoError(err, "failed to register pair")
				ctx = s.network.GetContext()
				id := s.network.App.GetErc20Keeper().GetTokenPairID(ctx, contractAddr.String())
				pair, _ = s.network.App.GetErc20Keeper().GetTokenPair(ctx, id)
				s.Require().NotNil(pair)

				// self destruct the token
				err = s.network.App.GetEVMKeeper().DeleteAccount(s.network.GetContext(), contractAddr)
				s.Require().NoError(err)

				sender = sdk.AccAddress(senderPk.PubKey().Address())

				// Fund receiver account with ATOM, ERC20 coins and IBC vouchers
				// We do this since we are interested in the conversion portion w/ OnRecvPacket
				err = testutil.FundAccount(
					ctx,
					s.network.App.GetBankKeeper(),
					sender,
					sdk.NewCoins(
						sdk.NewCoin(pair.Denom, math.NewInt(100)),
					),
				)
				s.Require().NoError(err)

				ack = channeltypes.NewErrorAcknowledgement(errors.New("error"))
				data = transfertypes.NewFungibleTokenPacketData(pair.Denom, "100", sender.String(), receiver.String(), "")
			},
			expERC20: big.NewInt(0),
			expPass:  false,
			expErrorEvents: func() {
				event := ctx.EventManager().Events()[len(ctx.EventManager().Events())-1]
				s.Require().Equal(event.Type, types.EventTypeFailedConvertERC20)
			},
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()

			tc.malleate()

			err := s.network.App.GetErc20Keeper().OnAcknowledgementPacket(
				ctx, channeltypes.Packet{}, data, ack,
			)

			if tc.expPass {
				s.Require().NoError(err)
				// check balance is the same as expected
				balance := s.network.App.GetErc20Keeper().BalanceOf(
					ctx, contracts.ERC20MinterBurnerDecimalsContract.ABI,
					pair.GetERC20Contract(),
					common.BytesToAddress(sender.Bytes()),
				)
				s.Require().Equal(tc.expERC20.Int64(), balance.Int64())
			} else {
				tc.expErrorEvents()
			}
		})
	}
}

func (s *KeeperTestSuite) TestOnTimeoutPacket() {
	var (
		ctx  sdk.Context
		data transfertypes.FungibleTokenPacketData
		pair types.TokenPair
	)
	senderPk := secp256k1.GenPrivKey()
	sender := sdk.AccAddress(senderPk.PubKey().Address())
	receiverPk := secp256k1.GenPrivKey()
	receiver := sdk.AccAddress(receiverPk.PubKey().Address())

	testCases := []struct {
		name           string
		malleate       func()
		expERC20       *big.Int
		expPass        bool
		expErrorEvents func()
	}{
		{
			name: "convert - pass timeout",
			malleate: func() {
				// Register Token Pair for testing
				contractAddr, err := s.setupRegisterERC20Pair(contractMinterBurner)
				s.Require().NoError(err, "failed to register pair")
				ctx = s.network.GetContext()
				id := s.network.App.GetErc20Keeper().GetTokenPairID(ctx, contractAddr.String())
				pair, _ = s.network.App.GetErc20Keeper().GetTokenPair(ctx, id)
				s.Require().NotNil(pair)

				stateDB := statedb.New(ctx, s.network.App.GetEVMKeeper(), statedb.NewEmptyTxConfig())
				_, err = s.network.App.GetEVMKeeper().CallEVM(ctx, stateDB, contracts.ERC20MinterBurnerDecimalsContract.ABI, s.keyring.GetAddr(0), contractAddr, true, false, nil, "mint", types.ModuleAddress, big.NewInt(100))
				s.Require().NoError(err)

				// Fund module account with ATOM, ERC20 coins and IBC vouchers
				// We do this since we are interested in the conversion portion w/ OnRecvPacket
				err = testutil.FundAccount(
					ctx,
					s.network.App.GetBankKeeper(),
					sender,
					sdk.NewCoins(
						sdk.NewCoin(pair.Denom, math.NewInt(100)),
					),
				)
				s.Require().NoError(err)

				data = transfertypes.NewFungibleTokenPacketData(pair.Denom, "10", sender.String(), receiver.String(), "")
			},
			expERC20:       big.NewInt(10),
			expPass:        true,
			expErrorEvents: func() {},
		},
		{
			name: "no-op - sender is module account",
			malleate: func() {
				// Register Token Pair for testing
				contractAddr, err := s.setupRegisterERC20Pair(contractMinterBurner)
				s.Require().NoError(err, "failed to register pair")
				ctx = s.network.GetContext()
				id := s.network.App.GetErc20Keeper().GetTokenPairID(ctx, contractAddr.String())
				pair, _ = s.network.App.GetErc20Keeper().GetTokenPair(ctx, id)
				s.Require().NotNil(pair)

				// any module account can be passed here
				moduleAcc := s.network.App.GetAccountKeeper().GetModuleAccount(ctx, evmtypes.ModuleName)

				data = transfertypes.NewFungibleTokenPacketData(pair.Denom, "10", moduleAcc.GetAddress().String(), "", "")
			},
			expERC20:       big.NewInt(0),
			expPass:        true,
			expErrorEvents: func() {},
		},
		{
			name: "err - self-destructed contract",
			malleate: func() {
				// Register Token Pair for testing
				contractAddr, err := s.setupRegisterERC20Pair(contractMinterBurner)
				s.Require().NoError(err, "failed to register pair")
				ctx = s.network.GetContext()
				id := s.network.App.GetErc20Keeper().GetTokenPairID(ctx, contractAddr.String())
				pair, _ = s.network.App.GetErc20Keeper().GetTokenPair(ctx, id)
				s.Require().NotNil(pair)
				// self destruct the token
				err = s.network.App.GetEVMKeeper().DeleteAccount(s.network.GetContext(), contractAddr)
				s.Require().NoError(err)

				// Fund receiver account with ATOM, ERC20 coins and IBC vouchers
				// We do this since we are interested in the conversion portion w/ OnRecvPacket
				err = testutil.FundAccount(
					ctx,
					s.network.App.GetBankKeeper(),
					sender,
					sdk.NewCoins(
						sdk.NewCoin(pair.Denom, math.NewInt(100)),
					),
				)
				s.Require().NoError(err)

				data = transfertypes.NewFungibleTokenPacketData(pair.Denom, "100", sender.String(), receiver.String(), "")
			},
			expERC20: big.NewInt(0),
			expPass:  false,
			expErrorEvents: func() {
				event := ctx.EventManager().Events()[len(ctx.EventManager().Events())-1]
				s.Require().Equal(event.Type, types.EventTypeFailedConvertERC20)
			},
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest()
			ctx = s.network.GetContext()

			tc.malleate()

			err := s.network.App.GetErc20Keeper().OnTimeoutPacket(ctx, channeltypes.Packet{}, data)
			if tc.expPass {
				s.Require().NoError(err)
				// check balance is the same as expected
				balance := s.network.App.GetErc20Keeper().BalanceOf(
					ctx, contracts.ERC20MinterBurnerDecimalsContract.ABI,
					pair.GetERC20Contract(),
					common.BytesToAddress(sender.Bytes()),
				)
				s.Require().Equal(tc.expERC20.Int64(), balance.Int64())
			} else {
				tc.expErrorEvents()
			}
		})
	}
}
