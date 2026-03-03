package keeper

import (
	"github.com/cosmos/evm/x/feesponsor/types"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Keeper grants access to the Fee Sponsor module state.
type Keeper struct {
	// Protobuf codec
	cdc codec.BinaryCodec
	// Store key required for the Fee Sponsor Prefix KVStore.
	storeKey storetypes.StoreKey
	// the address capable of executing a MsgSetFeePayer/MsgUpdateFeePayer message.
	// Typically, this should be the x/gov module account.
	authority sdk.AccAddress
}

// NewKeeper generates new fee sponsor module keeper
func NewKeeper(
	cdc codec.BinaryCodec,
	authority sdk.AccAddress,
	storeKey storetypes.StoreKey,
) Keeper {
	// ensure authority account is correctly formatted
	if err := sdk.VerifyAddressFormat(authority); err != nil {
		panic(err)
	}

	return Keeper{
		cdc:       cdc,
		storeKey:  storeKey,
		authority: authority,
	}
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", types.ModuleName)
}

// GetAuthority returns the module's authority.
func (k Keeper) GetAuthority() sdk.AccAddress {
	return k.authority
}

// SetFeePayerToStore sets the EVM fee payer to the store.
func (k Keeper) SetFeePayerToStore(ctx sdk.Context, feePayerAddr []byte) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.KeyPrefixFeePayer, feePayerAddr)
}

// GetFeePayer returns the EVM fee payer from the store.
func (k Keeper) GetFeePayer(ctx sdk.Context) ([]byte, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.KeyPrefixFeePayer)
	if bz == nil {
		return []byte{}, false
	}
	return bz, true
}

// RemoveFeePayerFromStore removes the EVM fee payer from the store.
func (k Keeper) RemoveFeePayerFromStore(ctx sdk.Context) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(types.KeyPrefixFeePayer)
}
