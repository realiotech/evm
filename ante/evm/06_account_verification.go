package evm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	"github.com/cosmos/evm/x/vm/keeper"
	"github.com/cosmos/evm/x/vm/statedb"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

// VerifyAccount checks that the account is valid and is an EOA (Externally Owned Account).
// The account will be set to store if it doesn't exist, i.e. cannot be found on store.
// This method will fail if:
// - from address is NOT an EOA (unless it's an EIP-7702 delegated account)
// Returns the account (either the input account or newly created empty account).
func VerifyAccount(
	ctx sdk.Context,
	evmKeeper anteinterfaces.EVMKeeper,
	accountKeeper anteinterfaces.AccountKeeper,
	account *statedb.Account,
	from common.Address,
) (*statedb.Account, error) {
	// Only EOA are allowed to send transactions.
	if account != nil && account.HasCodeHash() {
		// check eip-7702
		code := evmKeeper.GetCode(ctx, common.BytesToHash(account.CodeHash))
		_, delegated := ethtypes.ParseDelegation(code)
		if len(code) > 0 && !delegated {
			return nil, errorsmod.Wrapf(
				errortypes.ErrInvalidType,
				"the sender is not EOA: address %s", from,
			)
		}
	}
	if account == nil {
		acc := accountKeeper.NewAccountWithAddress(ctx, from.Bytes())
		accountKeeper.SetAccount(ctx, acc)
		account = statedb.NewEmptyAccount()
	}

	return account, nil
}

// VerifyAccountBalance checks that the account balance is greater than the total transaction cost.
// This method will fail if:
// - account balance is lower than the transaction cost
func VerifyAccountBalance(
	account *statedb.Account,
	cost *big.Int,
) error {
	if err := keeper.CheckSenderBalance(sdkmath.NewIntFromBigInt(account.Balance.ToBig()), cost); err != nil {
		return errorsmod.Wrap(err, "failed to check sender balance")
	}

	return nil
}
