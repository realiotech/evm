package cmd

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
)

func TestSetDefaultBlockMaxGas(t *testing.T) {
	genFile := filepath.Join(t.TempDir(), "genesis.json")
	require.NoError(t, genutil.ExportGenesisFileWithTime(genFile, "test-1", nil, json.RawMessage("{}"), time.Now()))

	appGenesis, err := genutiltypes.AppGenesisFromFile(genFile)
	require.NoError(t, err)
	require.Equal(t, int64(-1), appGenesis.Consensus.Params.Block.MaxGas)

	require.NoError(t, setDefaultBlockMaxGas(genFile))

	appGenesis, err = genutiltypes.AppGenesisFromFile(genFile)
	require.NoError(t, err)
	require.Equal(t, int64(60_000_000), appGenesis.Consensus.Params.Block.MaxGas)
}

func TestParseValidatorPowers(t *testing.T) {
	tests := []struct {
		name          string
		powers        []int
		numValidators int
		want          []int64
		wantErr       bool
	}{
		{
			name:          "empty slice - use defaults",
			powers:        []int{},
			numValidators: 4,
			want:          []int64{100, 100, 100, 100},
			wantErr:       false,
		},
		{
			name:          "nil slice - use defaults",
			powers:        nil,
			numValidators: 4,
			want:          []int64{100, 100, 100, 100},
			wantErr:       false,
		},
		{
			name:          "exact number of powers",
			powers:        []int{100, 200, 150, 300},
			numValidators: 4,
			want:          []int64{100, 200, 150, 300},
			wantErr:       false,
		},
		{
			name:          "fewer powers than validators",
			powers:        []int{100, 200},
			numValidators: 5,
			want:          []int64{100, 200, 200, 200, 200},
			wantErr:       false,
		},
		{
			name:          "single power for all validators",
			powers:        []int{500},
			numValidators: 3,
			want:          []int64{500, 500, 500},
			wantErr:       false,
		},
		{
			name:          "more powers than validators",
			powers:        []int{100, 200, 300, 400},
			numValidators: 2,
			want:          []int64{100, 200},
			wantErr:       false,
		},
		{
			name:          "invalid power - negative",
			powers:        []int{100, -200, 300},
			numValidators: 3,
			want:          nil,
			wantErr:       true,
		},
		{
			name:          "invalid power - zero",
			powers:        []int{100, 0, 300},
			numValidators: 3,
			want:          nil,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseValidatorPowers(tt.powers, tt.numValidators)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}
		})
	}
}
