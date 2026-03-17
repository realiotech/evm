package keeper

import (
	"context"

	"github.com/cosmos/evm/x/feesponsor/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ types.QueryServer = Keeper{}

// FeePayer returns the current EVM fee payer
func (k Keeper) FeePayer(goCtx context.Context, _ *types.QueryFeePayerRequest) (*types.QueryFeePayerResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	feePayer, found := k.GetFeePayer(ctx)
	if !found {
		return &types.QueryFeePayerResponse{}, types.ErrFeePayerNotSet
	}

	return &types.QueryFeePayerResponse{
		FeePayer: sdk.AccAddress(feePayer).String(),
	}, nil
}

