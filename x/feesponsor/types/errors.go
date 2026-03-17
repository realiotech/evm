package types

import (
	errorsmod "cosmossdk.io/errors"
)

var (
	ErrInvalidFeePayer = errorsmod.Register(ModuleName, 2, "invalid fee payer address")
	ErrFeePayerNotSet  = errorsmod.Register(ModuleName, 3, "fee payer not set")
)

