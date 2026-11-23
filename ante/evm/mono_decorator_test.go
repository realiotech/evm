package evm_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/cosmos/evm/ante/evm"
	"github.com/cosmos/evm/config"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/encoding"
	utiltx "github.com/cosmos/evm/testutil/tx"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/cosmos/evm/x/vm/statedb"
	evmsdktypes "github.com/cosmos/evm/x/vm/types"
	vmtypes "github.com/cosmos/evm/x/vm/types/mocks"
	"github.com/cosmos/gogoproto/proto"

	"cosmossdk.io/core/address"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	feesponsorkeeper "github.com/cosmos/evm/x/feesponsor/keeper"
)

// adds missing methods
type ExtendedEVMKeeper struct {
	*vmtypes.EVMKeeper
	// anteVerifiedSendersByTxHash holds every sender the decorator recorded.
	anteVerifiedSendersByTxHash map[common.Hash]common.Address
}

func NewExtendedEVMKeeper() *ExtendedEVMKeeper {
	return &ExtendedEVMKeeper{
		EVMKeeper:                   vmtypes.NewEVMKeeper(),
		anteVerifiedSendersByTxHash: map[common.Hash]common.Address{},
	}
}

func (k *ExtendedEVMKeeper) SetAnteVerifiedSender(_ sdk.Context, ethTxHash common.Hash, verifiedSender common.Address) {
	k.anteVerifiedSendersByTxHash[ethTxHash] = verifiedSender
}

func (k *ExtendedEVMKeeper) NewEVM(_ sdk.Context, _ core.Message, _ *statedb.EVMConfig, _ *tracing.Hooks, _ vm.StateDB) *vm.EVM {
	return nil
}

func (k *ExtendedEVMKeeper) DeductTxCostsFromUserBalance(_ sdk.Context, _ sdk.Coins, _ common.Address) error {
	return nil
}

func (k *ExtendedEVMKeeper) SpendableCoin(ctx sdk.Context, addr common.Address) *uint256.Int {
	account := k.GetAccount(ctx, addr)
	if account != nil {
		return account.Balance
	}
	return uint256.NewInt(0)
}

func (k *ExtendedEVMKeeper) ResetTransientGasUsed(_ sdk.Context) {}
func (k *ExtendedEVMKeeper) GetParams(_ sdk.Context) evmsdktypes.Params {
	return evmsdktypes.DefaultParams()
}
func (k *ExtendedEVMKeeper) GetBaseFee(_ sdk.Context) *big.Int           { return big.NewInt(0) }
func (k *ExtendedEVMKeeper) GetMinGasPrice(_ sdk.Context) math.LegacyDec { return math.LegacyZeroDec() }
func (k *ExtendedEVMKeeper) GetTxIndexTransient(_ sdk.Context) uint64    { return 0 }

// only methods called by EVMMonoDecorator
type MockFeeMarketKeeper struct{}

func (m MockFeeMarketKeeper) GetParams(ctx sdk.Context) feemarkettypes.Params {
	param := feemarkettypes.DefaultParams()
	param.BaseFee = m.GetBaseFee(ctx)
	return param
}

func (m MockFeeMarketKeeper) AddTransientGasWanted(_ sdk.Context, _ uint64) (uint64, error) {
	return 0, nil
}
func (m MockFeeMarketKeeper) GetBaseFeeEnabled(_ sdk.Context) bool    { return true }
func (m MockFeeMarketKeeper) GetBaseFee(_ sdk.Context) math.LegacyDec { return math.LegacyZeroDec() }

// matches the actual signatures
type MockAccountKeeper struct {
	FundedAddr sdk.AccAddress
}

func (m MockAccountKeeper) GetAccount(_ context.Context, addr sdk.AccAddress) sdk.AccountI {
	if m.FundedAddr != nil && addr.Equals(m.FundedAddr) {
		return &authtypes.BaseAccount{Address: addr.String()}
	}
	return nil
}
func (m MockAccountKeeper) SetAccount(_ context.Context, _ sdk.AccountI) {}
func (m MockAccountKeeper) NewAccountWithAddress(_ context.Context, _ sdk.AccAddress) sdk.AccountI {
	return nil
}
func (m MockAccountKeeper) RemoveAccount(_ context.Context, _ sdk.AccountI) {}
func (m MockAccountKeeper) GetModuleAddress(_ string) sdk.AccAddress        { return sdk.AccAddress{} }
func (m MockAccountKeeper) GetParams(_ context.Context) authtypes.Params {
	return authtypes.DefaultParams()
}

func (m MockAccountKeeper) GetSequence(_ context.Context, _ sdk.AccAddress) (uint64, error) {
	return 0, nil
}
func (m MockAccountKeeper) RemoveExpiredUnorderedNonces(_ sdk.Context) error { return nil }
func (m MockAccountKeeper) TryAddUnorderedNonce(_ sdk.Context, _ []byte, _ time.Time) error {
	return nil
}
func (m MockAccountKeeper) UnorderedTransactionsEnabled() bool { return false }
func (m MockAccountKeeper) AddressCodec() address.Codec        { return nil }

func signMsgEthereumTx(t *testing.T, privKey *ethsecp256k1.PrivKey, args *evmsdktypes.EvmTxArgs) *evmsdktypes.MsgEthereumTx {
	t.Helper()
	msg := evmsdktypes.NewTx(args)
	fromAddr := common.BytesToAddress(privKey.PubKey().Address().Bytes())
	msg.From = fromAddr.Bytes()
	ethSigner := ethtypes.LatestSignerForChainID(evmsdktypes.GetEthChainConfig().ChainID)
	require.NoError(t, msg.Sign(ethSigner, utiltx.NewSigner(privKey)))
	return msg
}

func setupFundedKeeper(t *testing.T, privKey *ethsecp256k1.PrivKey) (*ExtendedEVMKeeper, sdk.AccAddress) {
	t.Helper()
	fromAddr := common.BytesToAddress(privKey.PubKey().Address().Bytes())
	cosmosAddr := sdk.AccAddress(fromAddr.Bytes())
	keeper := NewExtendedEVMKeeper()
	fundedAccount := statedb.NewEmptyAccount()
	fundedAccount.Balance = uint256.MustFromDecimal("1000000000000000000") // 1 eth in wei
	require.NoError(t, keeper.SetAccount(sdk.Context{}, fromAddr, *fundedAccount))
	return keeper, cosmosAddr
}

func toMsgSlice(msgs []*evmsdktypes.MsgEthereumTx) []sdk.Msg {
	out := make([]sdk.Msg, len(msgs))
	for i, m := range msgs {
		out[i] = m
	}
	return out
}

// unsignedTxBodyPadding is 700 KB under field 1024, which TxBody does not define.
// The field number is non-critical, so the tx decoder accepts the bytes.
var unsignedTxBodyPadding = protowire.AppendBytes(protowire.AppendTag(nil, 1024, protowire.BytesType), make([]byte, 700_000))

func TestMonoDecorator(t *testing.T) {
	chainID := uint64(config.EighteenDecimalsChainID)
	cfg := encoding.MakeConfig(chainID)
	prepareTx := func(unsignedTxBodyBytes []byte, ethMsgs ...*evmsdktypes.MsgEthereumTx) (sdk.Tx, []byte) {
		tx, err := utiltx.PrepareEthTx(cfg.TxConfig, nil, toMsgSlice(ethMsgs)...)
		require.NoError(t, err)
		txBytes, err := cfg.TxConfig.TxEncoder()(tx)
		require.NoError(t, err)

		// The Ethereum signature does not cover the TxBody, so anyone can append to it.
		var rawTx txtypes.TxRaw
		require.NoError(t, proto.Unmarshal(txBytes, &rawTx))
		rawTx.BodyBytes = append(rawTx.BodyBytes, unsignedTxBodyBytes...)
		txBytes, err = proto.Marshal(&rawTx)
		require.NoError(t, err)
		return tx, txBytes
	}

	testCases := []struct {
		name     string
		simulate bool
		buildTx  func(privKey *ethsecp256k1.PrivKey) (sdk.Tx, []byte)
		expErr   string
	}{
		{
			"success with one evm tx",
			true,
			func(privKey *ethsecp256k1.PrivKey) (sdk.Tx, []byte) {
				args := &evmsdktypes.EvmTxArgs{
					Nonce:    0,
					GasLimit: 100000,
					GasPrice: big.NewInt(1),
					Input:    []byte("test"),
				}
				return prepareTx(nil, signMsgEthereumTx(t, privKey, args))
			},
			"",
		},
		{
			"failure with two evm txs",
			true,
			func(privKey *ethsecp256k1.PrivKey) (sdk.Tx, []byte) {
				args1 := &evmsdktypes.EvmTxArgs{
					Nonce:    0,
					GasLimit: 100000,
					GasPrice: big.NewInt(1),
					Input:    []byte("test"),
				}
				args2 := &evmsdktypes.EvmTxArgs{
					Nonce:    1,
					GasLimit: 100000,
					GasPrice: big.NewInt(1),
					Input:    []byte("test2"),
				}
				return prepareTx(nil,
					signMsgEthereumTx(t, privKey, args1),
					signMsgEthereumTx(t, privKey, args2),
				)
			},
			"expected 1 message, got 2",
		},
		{
			"failure with unsigned bytes padding the tx body",
			false,
			func(privKey *ethsecp256k1.PrivKey) (sdk.Tx, []byte) {
				args := &evmsdktypes.EvmTxArgs{
					Nonce:    0,
					GasLimit: 100000,
					GasPrice: big.NewInt(1),
					Input:    []byte("test"),
				}
				return prepareTx(unsignedTxBodyPadding, signMsgEthereumTx(t, privKey, args))
			},
			"ethereum tx envelope is not canonically encoded",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configurator := evmsdktypes.NewEVMConfigurator()
			configurator.ResetTestConfig()
			chainConfig := evmsdktypes.DefaultChainConfig(evmsdktypes.DefaultEVMChainID)
			err := evmsdktypes.SetChainConfig(chainConfig)
			require.NoError(t, err)
			coinInfo := evmsdktypes.EvmCoinInfo{
				Denom:         evmsdktypes.DefaultEVMExtendedDenom,
				ExtendedDenom: evmsdktypes.DefaultEVMExtendedDenom,
				DisplayDenom:  evmsdktypes.DefaultEVMDisplayDenom,
				Decimals:      18,
			}
			err = configurator.
				WithExtendedEips(evmsdktypes.DefaultCosmosEVMActivators).
				// NOTE: we're using the 18 decimals default for the example chain
				WithEVMCoinInfo(coinInfo).
				Configure()
			require.NoError(t, err)
			privKey, _ := ethsecp256k1.GenerateKey()
			keeper, cosmosAddr := setupFundedKeeper(t, privKey)
			accountKeeper := MockAccountKeeper{FundedAddr: cosmosAddr}
			feeMarketKeeper := MockFeeMarketKeeper{}
			params := keeper.GetParams(sdk.Context{})
			feemarketParams := feeMarketKeeper.GetParams(sdk.Context{})
			feegrantKeeper := feegrantkeeper.Keeper{}
			feesponsorKeeper := feesponsorkeeper.Keeper{}
			monoDec := evm.NewEVMMonoDecorator(accountKeeper, feeMarketKeeper, keeper, feegrantKeeper, feesponsorKeeper, 0, &params, &feemarketParams)
			ctx := sdk.NewContext(nil, tmproto.Header{}, false, log.NewNopLogger())
			ctx = ctx.WithBlockGasMeter(storetypes.NewGasMeter(1e19))

			tx, txBytes := tc.buildTx(privKey)

			newCtx, err := monoDec.AnteHandle(ctx.WithTxBytes(txBytes), tx, tc.simulate, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) { return ctx, nil })
			if tc.expErr == "" {
				require.NoError(t, err)
				require.NotNil(t, newCtx)

				// the tx that passed is recorded for the msg server, under its signer
				signerAddr := common.BytesToAddress(privKey.PubKey().Address().Bytes())
				ethMsg := tx.GetMsgs()[0].(*evmsdktypes.MsgEthereumTx)
				expectedVerifiedSenders := map[common.Hash]common.Address{ethMsg.Hash(): signerAddr}
				require.Equal(t, expectedVerifiedSenders, keeper.anteVerifiedSendersByTxHash)
			} else {
				require.ErrorContains(t, err, tc.expErr)
				require.Empty(t, keeper.anteVerifiedSendersByTxHash, "a rejected tx must not be recorded as verified")
			}
		})
	}
}

func TestValidateTxEncoding(t *testing.T) {
	// Only a coin with fewer than 18 decimals has an EVM denom that differs from
	// the extended denom the fee coin is converted to.
	coinConfigurator := evmsdktypes.NewEVMConfigurator()
	coinConfigurator.ResetTestConfig()
	require.NoError(t, evmsdktypes.SetChainConfig(evmsdktypes.DefaultChainConfig(evmsdktypes.DefaultEVMChainID)))
	sixDecimalsCoinInfo := evmsdktypes.EvmCoinInfo{Denom: "utest", ExtendedDenom: "atest", DisplayDenom: "test", Decimals: 6}
	require.NoError(t, coinConfigurator.WithEVMCoinInfo(sixDecimalsCoinInfo).Configure())

	txConfig := encoding.MakeConfig(uint64(config.EighteenDecimalsChainID)).TxConfig
	privKey, _ := ethsecp256k1.GenerateKey()
	recipientAddress := common.Address{}

	paidTransfer := signMsgEthereumTx(t, privKey, &evmsdktypes.EvmTxArgs{GasLimit: 21000, GasPrice: big.NewInt(7), To: &recipientAddress})
	freeTransfer := signMsgEthereumTx(t, privKey, &evmsdktypes.EvmTxArgs{GasLimit: 21000, GasPrice: big.NewInt(0), To: &recipientAddress})
	// Gas limit times gas price is wider than the 256 bits a coin amount holds.
	gasPriceOf255Bits := new(big.Int).Lsh(big.NewInt(1), 254)
	oversizedFeeTransfer := signMsgEthereumTx(t, privKey, &evmsdktypes.EvmTxArgs{GasLimit: 16_000_000, GasPrice: gasPriceOf255Bits, To: &recipientAddress})

	// BuildTx is how JSON-RPC and the mempool wrap a signed Ethereum tx.
	encodeBuiltTx := func(ethMsg *evmsdktypes.MsgEthereumTx) []byte {
		builtTx, err := ethMsg.BuildTx(txConfig.NewTxBuilder(), evmsdktypes.GetEVMCoinDenom())
		require.NoError(t, err)
		builtTxBytes, err := txConfig.TxEncoder()(builtTx)
		require.NoError(t, err)
		return builtTxBytes
	}
	paidTransferBytes := encodeBuiltTx(paidTransfer)

	// A timeout timestamp is a known TxBody field, so decoding and re-encoding
	// these bytes returns them unchanged; only the rebuilt envelope differs.
	var timedEnvelope txtypes.Tx
	require.NoError(t, proto.Unmarshal(paidTransferBytes, &timedEnvelope))
	timeoutTimestamp := time.Unix(4102444800, 0)
	timedEnvelope.Body.TimeoutTimestamp = &timeoutTimestamp
	timedTransferBytes, err := proto.Marshal(&timedEnvelope)
	require.NoError(t, err)

	testCases := []struct {
		name    string
		ethMsg  *evmsdktypes.MsgEthereumTx
		txBytes []byte
		expErr  string
	}{
		{"accepts the envelope BuildTx produces", paidTransfer, paidTransferBytes, ""},
		{"accepts the envelope BuildTx produces for a zero fee", freeTransfer, encodeBuiltTx(freeTransfer), ""},
		{"rejects an envelope with a timeout timestamp", paidTransfer, timedTransferBytes, "ethereum tx envelope is not canonically encoded"},
		{"rejects a fee wider than a coin amount", oversizedFeeTransfer, nil, "exceeds the maximum coin amount"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := evm.ValidateTxEncoding(testCase.ethMsg, testCase.txBytes)
			if testCase.expErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, testCase.expErr)
		})
	}
}
