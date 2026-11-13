package common

import (
	"testing"

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/vm/statedb"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type testKeeper struct {
	statedb.Keeper
	keys map[string]*storetypes.KVStoreKey
}

func (k testKeeper) KVStoreKeys() map[string]*storetypes.KVStoreKey {
	return k.keys
}

func TestRunNativeAction(t *testing.T) {
	t.Run("rejected call does not leak snapshot", func(t *testing.T) {
		keys := storetypes.NewKVStoreKeys("evm")
		tkeys := storetypes.NewTransientStoreKeys("evm_transient")
		ctx := testutil.DefaultContext(keys["evm"], tkeys["evm_transient"])

		keeper := testKeeper{keys: map[string]*storetypes.KVStoreKey{"evm": keys["evm"]}}
		stateDB := statedb.New(ctx, keeper, statedb.NewEmptyTxConfig())
		evm := vm.NewEVM(vm.BlockContext{}, stateDB, &params.ChainConfig{}, vm.Config{})
		evm.StateDB = stateDB

		contract := &vm.Contract{Gas: 10_000_000}
		p := Precompile{}
		action := func(_ sdk.Context) ([]byte, error) { return nil, nil }

		// exhaust the precompile call limit
		for i := uint64(0); i < vmtypes.MaxPrecompileCalls; i++ {
			_, err := p.RunNativeAction(evm, contract, action)
			require.NoError(t, err)
		}

		// baseline snapshot indices; consecutive snapshots differ by exactly one
		s0 := stateDB.MultiStoreSnapshot()
		s1 := stateDB.MultiStoreSnapshot()
		require.Equal(t, s0+1, s1)

		// every subsequent call is rejected by the limit check; none of them may
		// push an additional layer onto the snapshot cache stack
		for i := 0; i < 5; i++ {
			_, err := p.RunNativeAction(evm, contract, action)
			require.Error(t, err)
		}

		s2 := stateDB.MultiStoreSnapshot()
		require.Equal(t, s1+1, s2)
	})
}
