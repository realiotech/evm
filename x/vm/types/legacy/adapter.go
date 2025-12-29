package legacy

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// IsLegacy implements EthereumTxMsg - returns true for legacy format
func (msg *MsgEthereumTx) IsLegacy() bool {
	return true
}

// GetSenderAddr returns the sender as common.Address
// This is different from GetSender(chainID) which requires signature recovery
func (msg *MsgEthereumTx) GetSenderAddr() common.Address {
	if msg.From == "" {
		return common.Address{}
	}
	return common.HexToAddress(msg.From)
}

// GetFrom returns the sender address as sdk.AccAddress
func (msg *MsgEthereumTx) GetFrom() sdk.AccAddress {
	return msg.GetSenderAddr().Bytes()
}

// GetSenderLegacy returns the sender, falling back to signature recovery if needed
func (msg *MsgEthereumTx) GetSenderLegacy(signer ethtypes.Signer) (common.Address, error) {
	if msg.From != "" {
		return msg.GetSenderAddr(), nil
	}
	tx := msg.AsTransaction()
	if tx == nil {
		return common.Address{}, nil
	}
	from, err := ethtypes.Sender(signer, tx)
	if err != nil {
		return common.Address{}, err
	}
	msg.From = from.Hex()
	return from, nil
}

// GetHash returns the transaction hash
func (msg *MsgEthereumTx) GetHash() common.Hash {
	tx := msg.AsTransaction()
	if tx == nil {
		if msg.Hash != "" {
			return common.HexToHash(msg.Hash)
		}
		return common.Hash{}
	}
	return tx.Hash()
}

// GetGas returns gas limit
func (msg *MsgEthereumTx) GetGas() uint64 {
	tx := msg.AsTransaction()
	if tx == nil {
		return 0
	}
	return tx.Gas()
}

// GetGasPrice returns gas price
func (msg *MsgEthereumTx) GetGasPrice() *big.Int {
	tx := msg.AsTransaction()
	if tx == nil {
		return nil
	}
	return tx.GasPrice()
}

// GetGasFeeCap returns max fee per gas
func (msg *MsgEthereumTx) GetGasFeeCap() *big.Int {
	tx := msg.AsTransaction()
	if tx == nil {
		return nil
	}
	return tx.GasFeeCap()
}

// GetGasTipCap returns max priority fee
func (msg *MsgEthereumTx) GetGasTipCap() *big.Int {
	tx := msg.AsTransaction()
	if tx == nil {
		return nil
	}
	return tx.GasTipCap()
}

// GetValue returns transaction value
func (msg *MsgEthereumTx) GetValue() *big.Int {
	tx := msg.AsTransaction()
	if tx == nil {
		return nil
	}
	return tx.Value()
}

// GetNonce returns transaction nonce
func (msg *MsgEthereumTx) GetNonce() uint64 {
	tx := msg.AsTransaction()
	if tx == nil {
		return 0
	}
	return tx.Nonce()
}

// GetTo returns recipient address
func (msg *MsgEthereumTx) GetTo() *common.Address {
	tx := msg.AsTransaction()
	if tx == nil {
		return nil
	}
	return tx.To()
}

// GetInputData returns transaction input data
func (msg *MsgEthereumTx) GetInputData() []byte {
	tx := msg.AsTransaction()
	if tx == nil {
		return nil
	}
	return tx.Data()
}

