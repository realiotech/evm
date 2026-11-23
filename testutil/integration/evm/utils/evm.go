package utils

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ExecuteAnteVerifiedEthereumTx runs the given message through the
// MsgEthereumTx handler as if the EVM ante handler had verified and charged it
// for its From address, and returns the handler's response.
func ExecuteAnteVerifiedEthereumTx(
	ctx sdk.Context,
	evmKeeper *evmkeeper.Keeper,
	ethereumTxMsg *evmtypes.MsgEthereumTx,
) (*evmtypes.MsgEthereumTxResponse, error) {
	verifiedSender := common.BytesToAddress(ethereumTxMsg.From)
	evmKeeper.SetAnteVerifiedSender(ctx, ethereumTxMsg.Hash(), verifiedSender)

	return evmKeeper.EthereumTx(ctx, ethereumTxMsg)
}

// GetERC20Balance returns the token balance of a given account address for
// an ERC-20 token at the given contract address.
func GetERC20Balance(nw network.Network, tokenAddress, accountAddress common.Address) (*big.Int, error) {
	input, err := contracts.ERC20MinterBurnerDecimalsContract.ABI.Pack(
		"balanceOf",
		accountAddress,
	)
	if err != nil {
		return nil, err
	}

	callData, err := json.Marshal(evmtypes.TransactionArgs{
		To:    &tokenAddress,
		Input: (*hexutil.Bytes)(&input),
	})
	if err != nil {
		return nil, err
	}

	ethRes, err := nw.GetEvmClient().EthCall(
		nw.GetContext(),
		&evmtypes.EthCallRequest{
			Args: callData,
		},
	)
	if err != nil {
		return nil, err
	}

	fmt.Println("got ret: ", string(ethRes.Ret))
	fmt.Println("got eth call logs: ", ethRes.Logs)
	fmt.Println("got eth call error: ", ethRes.VmError)

	var balance *big.Int
	err = contracts.ERC20MinterBurnerDecimalsContract.ABI.UnpackIntoInterface(&balance, "balanceOf", ethRes.Ret)
	if err != nil {
		return nil, err
	}

	return balance, nil
}
