package keeper

import (
	"context"
	"fmt"

	"github.com/cosmos/evm/x/feesponsor/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

var _ types.MsgServer = &Keeper{}

// SetFeePayer sets the EVM fee payer address
func (k Keeper) SetFeePayer(goCtx context.Context, msg *types.MsgSetFeePayer) (*types.MsgSetFeePayerResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	fmt.Println("SetFeePayer")

	// Validate authority
	if err := k.validateAuthority(msg.Authority); err != nil {
		return nil, err
	}

	// Validate fee payer address
	addr, err := types.ValidateFeePayerAddress(msg.EvmFeePayer)
	if err != nil {
		return nil, err
	}

	// Set the fee payer
	k.SetFeePayerToStore(ctx, addr)

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.ModuleName,
			sdk.NewAttribute("action", "set_fee_payer"),
			sdk.NewAttribute("fee_payer", msg.EvmFeePayer),
		),
	)

	return &types.MsgSetFeePayerResponse{}, nil
}

// validateAuthority checks if the provided authority is the expected authority
func (k Keeper) validateAuthority(authority string) error {
	if authority != k.authority.String() {
		return errorsmod.Wrapf(
			govtypes.ErrInvalidSigner,
			"invalid authority; expected %s, got %s",
			k.authority.String(),
			authority,
		)
	}
	return nil
}
