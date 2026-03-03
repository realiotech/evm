package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	_ sdk.Msg = &MsgSetFeePayer{}
	_ sdk.Msg = &MsgRemoveFeePayer{}
)

// GetSigners returns the expected signers for MsgSetFeePayer
func (m *MsgSetFeePayer) GetSigners() []sdk.AccAddress {
	authority, err := sdk.AccAddressFromBech32(m.Authority)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{authority}
}

// ValidateBasic does a sanity check on the provided data
func (m *MsgSetFeePayer) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return errorsmod.Wrap(err, "invalid authority address")
	}

	if _, err := ValidateFeePayerAddress(m.EvmFeePayer); err != nil {
		return err
	}

	return nil
}

// ValidateBasic does a sanity check on the provided data
func (m *MsgRemoveFeePayer) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return errorsmod.Wrap(err, "invalid authority address")
	}

	return nil
}

// ValidateFeePayerAddress validates the fee payer address
func ValidateFeePayerAddress(address string) ([]byte, error) {
	if address == "" {
		return nil, errorsmod.Wrap(ErrInvalidFeePayer, "fee payer address cannot be empty")
	}

	// Validate as bech32 address
	addr, err := sdk.AccAddressFromBech32(address)
	if err != nil {
		return nil, errorsmod.Wrapf(ErrInvalidFeePayer, "invalid bech32 address: %s", err.Error())
	}

	return addr.Bytes(), nil
}
