package eip712_test

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/ethereum/eip712"

	"cosmossdk.io/math"
	txsigning "cosmossdk.io/x/tx/signing"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// buildProtobufSignDocBytes builds a SIGN_MODE_DIRECT SignDoc with the given
// fee granter and payer, encoded the same way clients and the verifier do.
func buildProtobufSignDocBytes(t *testing.T, granter, payer string, body *txtypes.TxBody) []byte {
	t.Helper()

	registry, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: protoregistry.GlobalFiles,
		SigningOptions: txsigning.Options{
			AddressCodec:          address.NewBech32Codec(sdk.Bech32PrefixAccAddr),
			ValidatorAddressCodec: address.NewBech32Codec(sdk.Bech32PrefixValAddr),
		},
	})
	require.NoError(t, err)
	cryptocodec.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	amino := codec.NewLegacyAmino()
	banktypes.RegisterLegacyAminoCodec(amino)
	eip712.SetEncodingConfig(amino, registry, 9000)

	// The protobuf sign doc path builds its amino-JSON sign bytes with
	// legacytx.StdSignBytes, which encodes messages with this global codec.
	// The app points it at its own codec in encoding.MakeConfig.
	previousSignBytesCodec := legacytx.RegressionTestingAminoCodec
	legacytx.RegressionTestingAminoCodec = amino
	t.Cleanup(func() { legacytx.RegressionTestingAminoCodec = previousSignBytesCodec })

	priv := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(priv.PubKey().Address())

	msgSend := banktypes.NewMsgSend(addr, addr, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(1))))
	msgAny, err := codectypes.NewAnyWithValue(msgSend)
	require.NoError(t, err)

	if body == nil {
		body = &txtypes.TxBody{Messages: []*codectypes.Any{msgAny}}
	}

	bodyBz, err := body.Marshal()
	require.NoError(t, err)

	pubKeyAny, err := codectypes.NewAnyWithValue(priv.PubKey())
	require.NoError(t, err)

	authInfoBz, err := (&txtypes.AuthInfo{
		SignerInfos: []*txtypes.SignerInfo{{
			PublicKey: pubKeyAny,
			ModeInfo: &txtypes.ModeInfo{Sum: &txtypes.ModeInfo_Single_{
				Single: &txtypes.ModeInfo_Single{Mode: 1}, // SIGN_MODE_DIRECT
			}},
		}},
		Fee: &txtypes.Fee{
			Amount:   sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(1000))),
			GasLimit: 100000,
			Granter:  granter,
			Payer:    payer,
		},
	}).Marshal()
	require.NoError(t, err)

	signDocBz, err := (&txtypes.SignDoc{
		BodyBytes:     bodyBz,
		AuthInfoBytes: authInfoBz,
		ChainId:       "9001",
	}).Marshal()
	require.NoError(t, err)

	return signDocBz
}

func TestGetEIP712TypedDataForMsgRejectsUnsupportedFields(t *testing.T) {
	now := time.Now().UTC()
	testCases := []struct {
		name    string
		granter string
		payer   string
		body    *txtypes.TxBody
		legacy  bool
	}{
		{"fee granter set", "cosmos1wq1r..." + "granter", "", nil, false},
		{"fee payer set", "", "cosmos1wq1r..." + "payer", nil, false},
		{"timeout timestamp set", "", "", &txtypes.TxBody{TimeoutTimestamp: &now}, false},
		{"unordered", "", "", &txtypes.TxBody{Unordered: true}, false},
		{"legacy fee granter set", "cosmos1v9zc6ggvr9582t7m4sd6rdc7d9tlsx9w76w7kt", "", nil, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			signDocBz := buildProtobufSignDocBytes(t, tc.granter, tc.payer, tc.body)

			var err error
			if tc.legacy {
				_, err = eip712.LegacyGetEIP712TypedDataForMsg(signDocBz, 9000)
			} else {
				_, err = eip712.GetEIP712TypedDataForMsg(signDocBz)
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), "body contains unsupported fields")
		})
	}
}

// buildMultiMsgSendBody returns a TxBody containing num MsgSend messages from a
// single signer.
func buildMultiMsgSendBody(t *testing.T, num int) *txtypes.TxBody {
	t.Helper()

	priv := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(priv.PubKey().Address())

	msgAnys := make([]*codectypes.Any, num)
	for i := range msgAnys {
		msgSend := banktypes.NewMsgSend(addr, addr, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(1))))
		msgAny, err := codectypes.NewAnyWithValue(msgSend)
		require.NoError(t, err)
		msgAnys[i] = msgAny
	}

	return &txtypes.TxBody{Messages: msgAnys}
}

func TestGetEIP712TypedDataForMsgRejectsTooManyMessages(t *testing.T) {
	testCases := []struct {
		name string
		num  int
	}{
		{"protobuf above limit", 33},
		{"legacy above limit", 33},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			signDocBz := buildProtobufSignDocBytes(t, "", "", buildMultiMsgSendBody(t, tc.num))

			var err error
			if strings.Contains(tc.name, "legacy") {
				_, err = eip712.LegacyGetEIP712TypedDataForMsg(signDocBz, 9000)
			} else {
				_, err = eip712.GetEIP712TypedDataForMsg(signDocBz)
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), "exceeds the maximum")
		})
	}
}

func TestGetEIP712TypedDataForMsgAcceptsMaxMessages(t *testing.T) {
	signDocBz := buildProtobufSignDocBytes(t, "", "", buildMultiMsgSendBody(t, 32))

	_, err := eip712.GetEIP712TypedDataForMsg(signDocBz)
	require.NoError(t, err)

	_, err = eip712.LegacyGetEIP712TypedDataForMsg(signDocBz, 9000)
	require.NoError(t, err)
}

// TestVerifySignatureLargeMultiSendBoundedWork checks that the EIP-712 fallback
// taken for an invalid signature does bounded work on a single message with a
// large nested collection, rather than rewriting the collection once per element.
func TestVerifySignatureLargeMultiSendBoundedWork(t *testing.T) {
	const numOutputs = 12000

	priv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	addr := sdk.AccAddress(priv.PubKey().Address())

	coin := sdk.NewCoins(sdk.NewCoin("atest", math.NewInt(1)))
	outputs := make([]banktypes.Output, numOutputs)
	for i := range outputs {
		outputs[i] = banktypes.NewOutput(addr, coin)
	}
	input := banktypes.NewInput(addr, sdk.NewCoins(sdk.NewCoin("atest", math.NewInt(numOutputs))))
	require.NoError(t, banktypes.ValidateInputOutputs(input, outputs))

	msgAny, err := codectypes.NewAnyWithValue(banktypes.NewMsgMultiSend(input, outputs))
	require.NoError(t, err)
	signDocBz := buildProtobufSignDocBytes(t, "", "", &txtypes.TxBody{Messages: []*codectypes.Any{msgAny}})

	// The payload is convertible, so the fallback performs the full conversion.
	_, err = eip712.GetEIP712TypedDataForMsg(signDocBz)
	require.NoError(t, err)

	pubKey := priv.PubKey()

	// Control: a valid signature over the same payload verifies.
	validSig, err := priv.Sign(signDocBz)
	require.NoError(t, err)
	require.True(t, pubKey.VerifySignature(signDocBz, validSig))

	// An invalid signature falls back to both EIP-712 encodings.
	invalidSig := make([]byte, 65)
	invalidSig[0] = 1

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	require.False(t, pubKey.VerifySignature(signDocBz, invalidSig))
	runtime.ReadMemStats(&after)

	// Quadratic conversion of this payload allocates tens of gigabytes; linear
	// conversion stays within a bounded multiple of the payload size.
	allocated := after.TotalAlloc - before.TotalAlloc
	require.Less(t, float64(allocated), 1000*float64(len(signDocBz)), "EIP-712 fallback allocated %d bytes for a %d byte payload", allocated, len(signDocBz))
}
