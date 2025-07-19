package backend

import (
	"fmt"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	rpctypes "github.com/cosmos/evm/rpc/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EthereumLikeMsg interface defines the contract for Ethereum-like messages
type EthereumLikeMsg interface {
	GetData() *codectypes.Any
	GetSender(chainID *big.Int) (common.Address, error)
	AsTransaction() *ethtypes.Transaction
	GetTxData() (evmtypes.TxData, error)
}

// ethereumMsgWrapper wraps MsgEthereumTx to implement EthereumLikeMsg interface
type ethereumMsgWrapper struct {
	*evmtypes.MsgEthereumTx
}

// reflectiveEthereumMsg wraps a message with reflection to implement EthereumLikeMsg
type reflectiveEthereumMsg struct {
	msg           sdk.Msg
	dataField     reflect.Value
	getSender     reflect.Value
	asTransaction reflect.Value
	getTxData     reflect.Value
}

func (r *reflectiveEthereumMsg) GetData() *codectypes.Any {
	return r.dataField.Interface().(*codectypes.Any)
}

func (r *reflectiveEthereumMsg) GetTxData() (evmtypes.TxData, error) {
	result := r.getTxData.Call(nil)
	if len(result) >= 2 {
		// Try to convert the result to the expected type
		resultInterface := result[0].Interface()
		// Check if it's already the right type
		if txData, ok := resultInterface.(evmtypes.TxData); ok {
			if err, ok := result[1].Interface().(error); ok && err != nil {
				return nil, err
			}
			return txData, nil
		}

		// If not, try to use the adapter
		if err, ok := result[1].Interface().(error); ok && err != nil {
			return nil, err
		}

		// Use the adapter to wrap the different TxData type
		return &txDataAdapter{data: resultInterface}, nil
	}
	return nil, fmt.Errorf("failed to get tx data from %T", r.msg)
}

func (r *reflectiveEthereumMsg) GetSender(chainID *big.Int) (common.Address, error) {
	result := r.getSender.Call([]reflect.Value{reflect.ValueOf(chainID)})
	if len(result) >= 2 {
		if addr, ok := result[0].Interface().(common.Address); ok {
			if err, ok := result[1].Interface().(error); ok && err != nil {
				return common.Address{}, err
			}
			return addr, nil
		}
	}
	return common.Address{}, fmt.Errorf("failed to get sender from %T", r.msg)
}

func (r *reflectiveEthereumMsg) AsTransaction() *ethtypes.Transaction {
	result := r.asTransaction.Call(nil)
	if len(result) > 0 {
		if tx, ok := result[0].Interface().(*ethtypes.Transaction); ok {
			return tx
		}
	}
	return nil
}

// extractEthereumLikeMsgReflective tries to extract an Ethereum-like message using reflection
func extractEthereumLikeMsgReflective(msg sdk.Msg) (EthereumLikeMsg, error) {
	// Try the current module's MsgEthereumTx first (most common case)
	if ethMsg, ok := msg.(*evmtypes.MsgEthereumTx); ok {
		return &ethereumMsgWrapper{ethMsg}, nil
	}

	// Use reflection to check if the message has the required fields and methods
	msgValue := reflect.ValueOf(msg)
	if msgValue.Kind() == reflect.Ptr {
		msgValue = msgValue.Elem()
	}

	// Check if it has the Data field
	dataField := msgValue.FieldByName("Data")
	if !dataField.IsValid() {
		return nil, fmt.Errorf("message type %T does not have Data field", msg)
	}

	// Check if it has the required methods
	getSenderMethod := msgValue.MethodByName("GetSender")
	asTransactionMethod := msgValue.MethodByName("AsTransaction")
	getTxDataMethod := msgValue.MethodByName("GetTxData")

	var missingMethods []string
	if !getSenderMethod.IsValid() {
		missingMethods = append(missingMethods, "GetSender")
	}
	if !asTransactionMethod.IsValid() {
		missingMethods = append(missingMethods, "AsTransaction")
	}
	if !getTxDataMethod.IsValid() {
		missingMethods = append(missingMethods, "GetTxData")
	}

	if len(missingMethods) == 0 {
		// Create a wrapper that implements EthereumLikeMsg
		return &reflectiveEthereumMsg{
			msg:           msg,
			dataField:     dataField,
			getSender:     getSenderMethod,
			asTransaction: asTransactionMethod,
			getTxData:     getTxDataMethod,
		}, nil
	}

	return nil, fmt.Errorf("message type %T is missing required Ethereum-like methods: %v", msg, missingMethods)
}

// txDataAdapter wraps different TxData types to provide a common interface
type txDataAdapter struct {
	data interface{}
}

func (a *txDataAdapter) GetGasPrice() *big.Int {
	// Use reflection to call GetGasPrice on the underlying data
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("GetGasPrice")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) > 0 {
			if gasPrice, ok := result[0].Interface().(*big.Int); ok {
				return gasPrice
			}
		}
	}
	return nil
}

func (a *txDataAdapter) GetGas() uint64 {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("GetGas")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) > 0 {
			if gas, ok := result[0].Interface().(uint64); ok {
				return gas
			}
		}
	}
	return 0
}

func (a *txDataAdapter) GetNonce() uint64 {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("GetNonce")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) > 0 {
			if nonce, ok := result[0].Interface().(uint64); ok {
				return nonce
			}
		}
	}
	return 0
}

func (a *txDataAdapter) GetTo() *common.Address {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("GetTo")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) > 0 {
			if to, ok := result[0].Interface().(*common.Address); ok {
				return to
			}
		}
	}
	return nil
}

func (a *txDataAdapter) GetAccessList() ethtypes.AccessList {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("GetAccessList")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) > 0 {
			if accessList, ok := result[0].Interface().(ethtypes.AccessList); ok {
				return accessList
			}
		}
	}
	return nil
}

func (a *txDataAdapter) GetChainID() *big.Int {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("GetChainID")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) > 0 {
			if chainID, ok := result[0].Interface().(*big.Int); ok {
				return chainID
			}
		}
	}
	return nil
}

func (a *txDataAdapter) GetData() []byte {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("GetData")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) > 0 {
			if data, ok := result[0].Interface().([]byte); ok {
				return data
			}
		}
	}
	return nil
}

func (a *txDataAdapter) GetGasFeeCap() *big.Int {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("GetGasFeeCap")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) > 0 {
			if gasFeeCap, ok := result[0].Interface().(*big.Int); ok {
				return gasFeeCap
			}
		}
	}
	return nil
}

func (a *txDataAdapter) GetGasTipCap() *big.Int {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("GetGasTipCap")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) > 0 {
			if gasTipCap, ok := result[0].Interface().(*big.Int); ok {
				return gasTipCap
			}
		}
	}
	return nil
}

func (a *txDataAdapter) GetValue() *big.Int {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("GetValue")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) > 0 {
			if value, ok := result[0].Interface().(*big.Int); ok {
				return value
			}
		}
	}
	return nil
}

// Required interface methods (minimal implementation)
func (a *txDataAdapter) TxType() byte {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("TxType")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) > 0 {
			if txType, ok := result[0].Interface().(byte); ok {
				return txType
			}
		}
	}
	return 0
}

func (a *txDataAdapter) Copy() evmtypes.TxData {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("Copy")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) > 0 {
			if copy, ok := result[0].Interface().(evmtypes.TxData); ok {
				return copy
			}
		}
	}
	return nil
}

func (a *txDataAdapter) GetRawSignatureValues() (v, r, s *big.Int) {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("GetRawSignatureValues")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) >= 3 {
			if vVal, ok := result[0].Interface().(*big.Int); ok {
				if rVal, ok := result[1].Interface().(*big.Int); ok {
					if sVal, ok := result[2].Interface().(*big.Int); ok {
						return vVal, rVal, sVal
					}
				}
			}
		}
	}
	return nil, nil, nil
}

func (a *txDataAdapter) SetSignatureValues(chainID, v, r, s *big.Int) {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("SetSignatureValues")
	if method.IsValid() {
		method.Call([]reflect.Value{
			reflect.ValueOf(chainID),
			reflect.ValueOf(v),
			reflect.ValueOf(r),
			reflect.ValueOf(s),
		})
	}
}

func (a *txDataAdapter) AsEthereumData() ethtypes.TxData {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("AsEthereumData")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) > 0 {
			if ethData, ok := result[0].Interface().(ethtypes.TxData); ok {
				return ethData
			}
		}
	}
	return nil
}

func (a *txDataAdapter) Validate() error {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("Validate")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) > 0 {
			if err, ok := result[0].Interface().(error); ok {
				return err
			}
		}
	}
	return nil
}

func (a *txDataAdapter) Fee() *big.Int {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("Fee")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) > 0 {
			if fee, ok := result[0].Interface().(*big.Int); ok {
				return fee
			}
		}
	}
	return nil
}

func (a *txDataAdapter) Cost() *big.Int {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("Cost")
	if method.IsValid() {
		result := method.Call(nil)
		if len(result) > 0 {
			if cost, ok := result[0].Interface().(*big.Int); ok {
				return cost
			}
		}
	}
	return nil
}

func (a *txDataAdapter) EffectiveGasPrice(baseFee *big.Int) *big.Int {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("EffectiveGasPrice")
	if method.IsValid() {
		result := method.Call([]reflect.Value{reflect.ValueOf(baseFee)})
		if len(result) > 0 {
			if effectivePrice, ok := result[0].Interface().(*big.Int); ok {
				return effectivePrice
			}
		}
	}
	return nil
}

func (a *txDataAdapter) EffectiveFee(baseFee *big.Int) *big.Int {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("EffectiveFee")
	if method.IsValid() {
		result := method.Call([]reflect.Value{reflect.ValueOf(baseFee)})
		if len(result) > 0 {
			if effectiveFee, ok := result[0].Interface().(*big.Int); ok {
				return effectiveFee
			}
		}
	}
	return nil
}

func (a *txDataAdapter) EffectiveCost(baseFee *big.Int) *big.Int {
	val := reflect.ValueOf(a.data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	method := val.MethodByName("EffectiveCost")
	if method.IsValid() {
		result := method.Call([]reflect.Value{reflect.ValueOf(baseFee)})
		if len(result) > 0 {
			if effectiveCost, ok := result[0].Interface().(*big.Int); ok {
				return effectiveCost
			}
		}
	}
	return nil
}

// NewTransactionFromEthereumLikeMsg returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
func NewTransactionFromEthereumLikeMsg(
	msg EthereumLikeMsg,
	blockHash common.Hash,
	blockNumber, index uint64,
	baseFee *big.Int,
	chainID *big.Int,
) (*rpctypes.RPCTransaction, error) {
	tx := msg.AsTransaction()
	return rpctypes.NewRPCTransaction(tx, blockHash, blockNumber, index, baseFee, chainID)
}
