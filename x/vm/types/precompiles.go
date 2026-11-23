package types

const (
	Bech32PrecompileAddress       = "0x0000000000000000000000000000000000000400"
	StakingPrecompileAddress      = "0x0000000000000000000000000000000000000800"
	DistributionPrecompileAddress = "0x0000000000000000000000000000000000000801"
	ICS20PrecompileAddress        = "0x0000000000000000000000000000000000000802"
	VestingPrecompileAddress      = "0x0000000000000000000000000000000000000803"
	BankPrecompileAddress         = "0x0000000000000000000000000000000000000804"
	GovPrecompileAddress          = "0x0000000000000000000000000000000000000805"
	SlashingPrecompileAddress     = "0x0000000000000000000000000000000000000806"
)

// AvailableStaticPrecompiles defines the full list of all available EVM extension addresses.
//
// NOTE: To be explicit, this list does not include the dynamically registered EVM extensions
// like the ERC-20 extensions.
var AvailableStaticPrecompiles = []string{
	Bech32PrecompileAddress,
	StakingPrecompileAddress,
	DistributionPrecompileAddress,
	ICS20PrecompileAddress,
	VestingPrecompileAddress,
	BankPrecompileAddress,
	GovPrecompileAddress,
	SlashingPrecompileAddress,
}
