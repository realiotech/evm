package types

// DefaultGenesisState sets default feesponsor genesis state
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		FeePayer: "",
	}
}

// Validate performs basic genesis state validation returning an error upon any failure.
func (gs GenesisState) Validate() error {
	// Validate fee payer address if set
	if gs.FeePayer != "" {
		if _, err := ValidateFeePayerAddress(gs.FeePayer); err != nil {
			return err
		}
	}
	return nil
}

