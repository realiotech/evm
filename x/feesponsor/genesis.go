package feesponsor

import (
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/evm/x/feesponsor/keeper"
	"github.com/cosmos/evm/x/feesponsor/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the feesponsor module's state from a provided genesis state.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) []abci.ValidatorUpdate {
	// Set fee payer if provided
	if genState.FeePayer != "" {
		addr, err := types.ValidateFeePayerAddress(genState.FeePayer)
		if err != nil {
			panic(err)
		}
		k.SetFeePayerToStore(ctx, addr)
	}

	return []abci.ValidatorUpdate{}
}

// ExportGenesis returns the feesponsor module's exported genesis.
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	feePayer, found := k.GetFeePayer(ctx)
	if !found {
		return types.DefaultGenesisState()
	}

	return &types.GenesisState{
		FeePayer: sdk.AccAddress(feePayer).String(),
	}
}
