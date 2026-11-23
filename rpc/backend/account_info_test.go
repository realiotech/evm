package backend

import (
	"context"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	rpctypes "github.com/cosmos/evm/rpc/types"
)

func TestGetProofStorageKeysLimit(t *testing.T) {
	keys := make([]string, maxGetProofKeys+1)
	for i := range keys {
		keys[i] = common.Hash{}.String()
	}

	b := &Backend{}
	_, err := b.GetProof(context.Background(), common.Address{}, keys, rpctypes.BlockNumberOrHash{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "too many storage keys requested")
	require.True(t, strings.Contains(err.Error(), "1024"), "expected limit of 1024 in error, got: %s", err)
}
