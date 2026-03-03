package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/feesponsor/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestSetFeePayer(t *testing.T) {
	testCases := []struct {
		name      string
		msg       *types.MsgSetFeePayer
		expectErr bool
		errMsg    string
	}{
		{
			name: "pass - valid set fee payer",
			msg: &types.MsgSetFeePayer{
				Authority:   sdk.AccAddress("authority").String(),
				EvmFeePayer: sdk.AccAddress("feepayer1").String(),
			},
			expectErr: false,
		},
		{
			name: "fail - invalid authority",
			msg: &types.MsgSetFeePayer{
				Authority:   sdk.AccAddress("invalid").String(),
				EvmFeePayer: sdk.AccAddress("feepayer1").String(),
			},
			expectErr: true,
			errMsg:    "invalid authority",
		},
		{
			name: "fail - empty fee payer address",
			msg: &types.MsgSetFeePayer{
				Authority:   sdk.AccAddress("authority").String(),
				EvmFeePayer: "",
			},
			expectErr: true,
			errMsg:    "fee payer address cannot be empty",
		},
		{
			name: "fail - invalid fee payer address",
			msg: &types.MsgSetFeePayer{
				Authority:   sdk.AccAddress("authority").String(),
				EvmFeePayer: "invalid_address",
			},
			expectErr: true,
			errMsg:    "invalid bech32 address",
		},
		{
			name: "pass - update existing fee payer",
			msg: &types.MsgSetFeePayer{
				Authority:   sdk.AccAddress("authority").String(),
				EvmFeePayer: sdk.AccAddress("feepayer2").String(),
			},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			td := newTestData(t)

			resp, err := td.keeper.SetFeePayer(td.ctx, tc.msg)

			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)

				// Verify fee payer was set
				addr, err := types.ValidateFeePayerAddress(tc.msg.EvmFeePayer)
				require.NoError(t, err)

				stored, found := td.keeper.GetFeePayer(td.ctx)
				require.True(t, found)
				require.Equal(t, addr, stored)
			}
		})
	}
}

func TestRemoveFeePayer(t *testing.T) {
	testCases := []struct {
		name      string
		setupFn   func(td testData)
		msg       *types.MsgRemoveFeePayer
		expectErr bool
		errMsg    string
	}{
		{
			name: "pass - remove existing fee payer",
			setupFn: func(td testData) {
				// Set a fee payer first
				addr := sdk.AccAddress("feepayer1")
				td.keeper.SetFeePayerToStore(td.ctx, addr)
			},
			msg: &types.MsgRemoveFeePayer{
				Authority: sdk.AccAddress("authority").String(),
			},
			expectErr: false,
		},
		{
			name:    "pass - remove non-existent fee payer",
			setupFn: func(td testData) {},
			msg: &types.MsgRemoveFeePayer{
				Authority: sdk.AccAddress("authority").String(),
			},
			expectErr: false,
		},
		{
			name: "fail - invalid authority",
			setupFn: func(td testData) {
				addr := sdk.AccAddress("feepayer1")
				td.keeper.SetFeePayerToStore(td.ctx, addr)
			},
			msg: &types.MsgRemoveFeePayer{
				Authority: sdk.AccAddress("invalid").String(),
			},
			expectErr: true,
			errMsg:    "invalid authority",
		},
		{
			name:    "fail - empty authority",
			setupFn: func(td testData) {},
			msg: &types.MsgRemoveFeePayer{
				Authority: "",
			},
			expectErr: true,
			errMsg:    "invalid authority",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			td := newTestData(t)
			tc.setupFn(td)

			resp, err := td.keeper.RemoveFeePayer(td.ctx, tc.msg)

			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)

				// Verify fee payer was removed
				_, found := td.keeper.GetFeePayer(td.ctx)
				require.False(t, found)
			}
		})
	}
}

func TestSetFeePayerEvent(t *testing.T) {
	td := newTestData(t)

	msg := &types.MsgSetFeePayer{
		Authority:   td.authority.String(),
		EvmFeePayer: sdk.AccAddress("feepayer1").String(),
	}

	resp, err := td.keeper.SetFeePayer(td.ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Check event was emitted
	events := td.ctx.EventManager().Events()
	require.NotEmpty(t, events)

	// Find the feesponsor event
	var found bool
	for _, event := range events {
		if event.Type == types.ModuleName {
			found = true
			// Check attributes
			attrs := event.Attributes
			require.Len(t, attrs, 2)
			require.Equal(t, "action", attrs[0].Key)
			require.Equal(t, "set_fee_payer", attrs[0].Value)
			require.Equal(t, "fee_payer", attrs[1].Key)
			require.Equal(t, msg.EvmFeePayer, attrs[1].Value)
		}
	}
	require.True(t, found, "feesponsor event not found")
}

func TestRemoveFeePayerEvent(t *testing.T) {
	td := newTestData(t)

	// Set a fee payer first
	addr := sdk.AccAddress("feepayer1")
	td.keeper.SetFeePayerToStore(td.ctx, addr)

	msg := &types.MsgRemoveFeePayer{
		Authority: td.authority.String(),
	}

	resp, err := td.keeper.RemoveFeePayer(td.ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Check event was emitted
	events := td.ctx.EventManager().Events()
	require.NotEmpty(t, events)

	// Find the feesponsor event
	var found bool
	for _, event := range events {
		if event.Type == types.ModuleName {
			found = true
			// Check attributes
			attrs := event.Attributes
			require.Len(t, attrs, 1)
			require.Equal(t, "action", attrs[0].Key)
			require.Equal(t, "remove_fee_payer", attrs[0].Value)
		}
	}
	require.True(t, found, "feesponsor event not found")
}

func TestSetFeePayerMultipleTimes(t *testing.T) {
	td := newTestData(t)

	// Set fee payer first time
	msg1 := &types.MsgSetFeePayer{
		Authority:   td.authority.String(),
		EvmFeePayer: sdk.AccAddress("feepayer1").String(),
	}
	resp1, err := td.keeper.SetFeePayer(td.ctx, msg1)
	require.NoError(t, err)
	require.NotNil(t, resp1)

	addr1, err := types.ValidateFeePayerAddress(msg1.EvmFeePayer)
	require.NoError(t, err)

	stored1, found := td.keeper.GetFeePayer(td.ctx)
	require.True(t, found)
	require.Equal(t, addr1, stored1)

	// Update fee payer
	msg2 := &types.MsgSetFeePayer{
		Authority:   td.authority.String(),
		EvmFeePayer: sdk.AccAddress("feepayer2").String(),
	}
	resp2, err := td.keeper.SetFeePayer(td.ctx, msg2)
	require.NoError(t, err)
	require.NotNil(t, resp2)

	addr2, err := types.ValidateFeePayerAddress(msg2.EvmFeePayer)
	require.NoError(t, err)

	stored2, found := td.keeper.GetFeePayer(td.ctx)
	require.True(t, found)
	require.Equal(t, addr2, stored2)
	require.NotEqual(t, addr1, addr2)
}

func TestRemoveFeePayerThenSet(t *testing.T) {
	td := newTestData(t)

	// Set fee payer
	setMsg := &types.MsgSetFeePayer{
		Authority:   td.authority.String(),
		EvmFeePayer: sdk.AccAddress("feepayer1").String(),
	}
	_, err := td.keeper.SetFeePayer(td.ctx, setMsg)
	require.NoError(t, err)

	// Verify it exists
	_, found := td.keeper.GetFeePayer(td.ctx)
	require.True(t, found)

	// Remove fee payer
	removeMsg := &types.MsgRemoveFeePayer{
		Authority: td.authority.String(),
	}
	_, err = td.keeper.RemoveFeePayer(td.ctx, removeMsg)
	require.NoError(t, err)

	// Verify it was removed
	_, found = td.keeper.GetFeePayer(td.ctx)
	require.False(t, found)

	// Set fee payer again
	setMsg2 := &types.MsgSetFeePayer{
		Authority:   td.authority.String(),
		EvmFeePayer: sdk.AccAddress("feepayer2").String(),
	}
	_, err = td.keeper.SetFeePayer(td.ctx, setMsg2)
	require.NoError(t, err)

	// Verify it exists
	addr, found := td.keeper.GetFeePayer(td.ctx)
	require.True(t, found)

	expectedAddr, err := types.ValidateFeePayerAddress(setMsg2.EvmFeePayer)
	require.NoError(t, err)
	require.Equal(t, expectedAddr, addr)
}
