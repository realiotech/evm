package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	evmencoding "github.com/cosmos/evm/encoding"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/x/feesponsor/keeper"
	"github.com/cosmos/evm/x/feesponsor/types"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// testData defines necessary fields for testing keeper store methods
type testData struct {
	ctx       sdk.Context
	keeper    keeper.Keeper
	storeKey  *storetypes.KVStoreKey
	authority sdk.AccAddress
}

// newTestData creates a new testData instance for unit tests
func newTestData(t *testing.T) testData {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.ModuleName)
	tKey := storetypes.NewTransientStoreKey("transient_test")
	ctx := testutil.DefaultContext(storeKey, tKey)

	chainID := testconstants.SixDecimalsChainID.EVMChainID
	cfg := evmencoding.MakeConfig(chainID)
	cdc := cfg.Codec

	authority := sdk.AccAddress("authority")
	k := keeper.NewKeeper(cdc, authority, storeKey)

	evmConfigurator := vmtypes.NewEVMConfigurator().
		WithEVMCoinInfo(testconstants.ExampleChainCoinInfo[testconstants.SixDecimalsChainID])
	evmConfigurator.ResetTestConfig()
	err := evmConfigurator.Configure()
	require.NoError(t, err)

	return testData{
		ctx:       ctx,
		keeper:    k,
		storeKey:  storeKey,
		authority: authority,
	}
}

func TestNewKeeper(t *testing.T) {
	td := newTestData(t)

	require.NotNil(t, td.keeper)
	require.Equal(t, td.authority, td.keeper.GetAuthority())
}

func TestSetFeePayerToStore(t *testing.T) {
	td := newTestData(t)

	testCases := []struct {
		name         string
		feePayerAddr []byte
	}{
		{
			name:         "set valid fee payer address",
			feePayerAddr: sdk.AccAddress("feepayer1"),
		},
		{
			name:         "set different fee payer address",
			feePayerAddr: sdk.AccAddress("feepayer2"),
		},
		{
			name:         "set empty fee payer address",
			feePayerAddr: []byte{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			td.keeper.SetFeePayerToStore(td.ctx, tc.feePayerAddr)

			// Verify it was stored
			stored, found := td.keeper.GetFeePayer(td.ctx)
			require.True(t, found)
			require.Equal(t, tc.feePayerAddr, stored)
		})
	}
}

func TestGetFeePayer(t *testing.T) {
	testCases := []struct {
		name        string
		setupFn     func(td testData)
		expectFound bool
		expectAddr  []byte
	}{
		{
			name: "get existing fee payer",
			setupFn: func(td testData) {
				addr := sdk.AccAddress("feepayer1")
				td.keeper.SetFeePayerToStore(td.ctx, addr)
			},
			expectFound: true,
			expectAddr:  sdk.AccAddress("feepayer1"),
		},
		{
			name:        "get non-existent fee payer",
			setupFn:     func(td testData) {},
			expectFound: false,
			expectAddr:  []byte{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fresh test data for each test case
			td := newTestData(t)
			tc.setupFn(td)

			addr, found := td.keeper.GetFeePayer(td.ctx)
			require.Equal(t, tc.expectFound, found)
			require.Equal(t, tc.expectAddr, addr)
		})
	}
}

func TestRemoveFeePayerFromStore(t *testing.T) {
	td := newTestData(t)

	testCases := []struct {
		name    string
		setupFn func()
	}{
		{
			name: "remove existing fee payer",
			setupFn: func() {
				addr := sdk.AccAddress("feepayer1")
				td.keeper.SetFeePayerToStore(td.ctx, addr)
			},
		},
		{
			name:    "remove non-existent fee payer",
			setupFn: func() {},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fresh test data for each test case
			td := newTestData(t)
			tc.setupFn()

			// Remove fee payer
			td.keeper.RemoveFeePayerFromStore(td.ctx)

			// Verify it was removed
			_, found := td.keeper.GetFeePayer(td.ctx)
			require.False(t, found)
		})
	}
}
