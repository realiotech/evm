// Portions of this file are derived from the go-ethereum project,
// Copyright 2017 The go-ethereum Authors
// (https://github.com/ethereum/go-ethereum).
// Used under the terms of the GNU Lesser General Public License v3.0.
//
// The modifications and additional code are licensed under the Apache License 2.0.
// See the accompanying LICENSE file for full terms.
//
// Original go-ethereum license: https://www.gnu.org/licenses/lgpl-3.0.html

package common

import (
	"bytes"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestMakeTopic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   interface{}
		want    common.Hash
		wantErr bool
	}{
		{
			name:  "common.Hash",
			input: common.HexToHash("0x01020304"),
			want: func() common.Hash {
				return common.HexToHash("0x01020304")
			}(),
			wantErr: false,
		},
		{
			name:  "common.Address",
			input: common.HexToAddress("0x0102030405060708090a0b0c0d0e0f1011121314"),
			want: func() common.Hash {
				var h common.Hash
				addr := common.HexToAddress("0x0102030405060708090a0b0c0d0e0f1011121314")
				copy(h[common.HashLength-common.AddressLength:], addr.Bytes())
				return h
			}(),
			wantErr: false,
		},
		{
			name: "positive *big.Int",
			input: func() *big.Int {
				return big.NewInt(123456789)
			}(),
			want: func() common.Hash {
				var h common.Hash
				blob := big.NewInt(123456789).Bytes()
				copy(h[common.HashLength-len(blob):], blob)
				return h
			}(),
			wantErr: false,
		},
		{
			name: "negative *big.Int (not sign-extended)",
			input: func() *big.Int {
				return big.NewInt(-1)
			}(),
			want: func() common.Hash {
				var h common.Hash
				// big.Int.Bytes() returns the absolute value; no two's complement
				blob := big.NewInt(-1).Bytes()
				copy(h[common.HashLength-len(blob):], blob)
				return h
			}(),
			wantErr: false,
		},
		{
			name:  "bool: true",
			input: true,
			want: func() common.Hash {
				var h common.Hash
				h[common.HashLength-1] = 1
				return h
			}(),
			wantErr: false,
		},
		{
			name:    "bool: false",
			input:   false,
			want:    common.Hash{},
			wantErr: false,
		},
		{
			name:  "int8",
			input: int8(-2),
			want: func() common.Hash {
				return [32]byte{
					255, 255, 255, 255, 255, 255, 255, 255,
					255, 255, 255, 255, 255, 255, 255, 255,
					255, 255, 255, 255, 255, 255, 255, 255,
					255, 255, 255, 255, 255, 255, 255, 254,
				}
			}(),
			wantErr: false,
		},
		{
			name:  "int16",
			input: int16(-1234),
			want: func() common.Hash {
				var h common.Hash
				// genIntType(-1234, 2) => two's complement for int16
				// you can verify by using the same method as genIntType in your code
				gen := genIntType(int64(int16(-1234)), 2)
				copy(h[:], gen)
				return h
			}(),
			wantErr: false,
		},
		{
			name:  "int32",
			input: int32(-56789),
			want: func() common.Hash {
				var h common.Hash
				// genIntType(-56789, 4)
				gen := genIntType(int64(int32(-56789)), 4)
				copy(h[:], gen)
				return h
			}(),
			wantErr: false,
		},
		{
			name:  "int64",
			input: int64(-5),
			want: func() common.Hash {
				return [32]byte{
					255, 255, 255, 255, 255, 255, 255, 255,
					255, 255, 255, 255, 255, 255, 255, 255,
					255, 255, 255, 255, 255, 255, 255, 255,
					255, 255, 255, 255, 255, 255, 255, 251,
				}
			}(),
			wantErr: false,
		},
		{
			name:  "uint8",
			input: uint8(255),
			want: func() common.Hash {
				var h common.Hash
				blob := new(big.Int).SetUint64(uint64(uint8(255))).Bytes()
				copy(h[common.HashLength-len(blob):], blob)
				return h
			}(),
			wantErr: false,
		},
		{
			name:  "uint16",
			input: uint16(65535),
			want: func() common.Hash {
				var h common.Hash
				blob := new(big.Int).SetUint64(uint64(uint16(65535))).Bytes()
				copy(h[common.HashLength-len(blob):], blob)
				return h
			}(),
			wantErr: false,
		},
		{
			name:  "uint32",
			input: uint32(4294967295),
			want: func() common.Hash {
				var h common.Hash
				blob := new(big.Int).SetUint64(uint64(uint32(4294967295))).Bytes()
				copy(h[common.HashLength-len(blob):], blob)
				return h
			}(),
			wantErr: false,
		},
		{
			name:  "uint64",
			input: uint64(4294967296),
			want: func() common.Hash {
				var h common.Hash
				blob := new(big.Int).SetUint64(4294967296).Bytes()
				copy(h[common.HashLength-len(blob):], blob)
				return h
			}(),
			wantErr: false,
		},
		{
			name:  "string",
			input: "hello world",
			want: func() common.Hash {
				// Strings are hashed using keccak256
				return crypto.Keccak256Hash([]byte("hello world"))
			}(),
			wantErr: false,
		},
		{
			name:  "[]byte",
			input: []byte{0x01, 0x02, 0x03},
			want: func() common.Hash {
				// Byte slices are hashed using keccak256
				return crypto.Keccak256Hash([]byte{0x01, 0x02, 0x03})
			}(),
			wantErr: false,
		},
		{
			name:  "[5]byte (static byte array)",
			input: [5]byte{0x01, 0x02, 0x03, 0x04, 0x05},
			want: func() common.Hash {
				var h common.Hash
				copy(h[0:5], []byte{0x01, 0x02, 0x03, 0x04, 0x05})
				return h
			}(),
			wantErr: false,
		},
		{
			name:    "unsupported type",
			input:   struct{ Foo string }{"bar"},
			want:    common.Hash{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		// Capture range variable for parallel tests
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := MakeTopic(tt.input)
			if tt.wantErr {
				require.Error(t, err, "expected error but got nil")
				return
			}
			require.NoError(t, err, "expected no error but got error")

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MakeTopic(%v) = \n%#v\nwant \n%#v", tt.input, got, tt.want)
			}
		})
	}
}

func TestPackNumber(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value  reflect.Value
		packed []byte
	}{
		// Protocol limits
		{reflect.ValueOf(0), common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000000")},
		{reflect.ValueOf(1), common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000001")},
		{reflect.ValueOf(-1), common.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")},

		// Type corner cases
		{reflect.ValueOf(uint8(math.MaxUint8)), common.Hex2Bytes("00000000000000000000000000000000000000000000000000000000000000ff")},
		{reflect.ValueOf(uint16(math.MaxUint16)), common.Hex2Bytes("000000000000000000000000000000000000000000000000000000000000ffff")},
		{reflect.ValueOf(uint32(math.MaxUint32)), common.Hex2Bytes("00000000000000000000000000000000000000000000000000000000ffffffff")},
		{reflect.ValueOf(uint64(math.MaxUint64)), common.Hex2Bytes("000000000000000000000000000000000000000000000000ffffffffffffffff")},

		{reflect.ValueOf(int8(math.MaxInt8)), common.Hex2Bytes("000000000000000000000000000000000000000000000000000000000000007f")},
		{reflect.ValueOf(int16(math.MaxInt16)), common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000007fff")},
		{reflect.ValueOf(int32(math.MaxInt32)), common.Hex2Bytes("000000000000000000000000000000000000000000000000000000007fffffff")},
		{reflect.ValueOf(int64(math.MaxInt64)), common.Hex2Bytes("0000000000000000000000000000000000000000000000007fffffffffffffff")},

		{reflect.ValueOf(int8(math.MinInt8)), common.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff80")},
		{reflect.ValueOf(int16(math.MinInt16)), common.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8000")},
		{reflect.ValueOf(int32(math.MinInt32)), common.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffffffffffff80000000")},
		{reflect.ValueOf(int64(math.MinInt64)), common.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffff8000000000000000")},
	}
	for i, tt := range tests {
		packed := PackNum(tt.value)
		if !bytes.Equal(packed, tt.packed) {
			t.Errorf("test %d: pack mismatch: have %x, want %x", i, packed, tt.packed)
		}
	}
}

// TestSetupABIDecodedSizeLimit verifies that SetupABI rejects inputs whose
// offsets alias a shared dynamic tail, so that the decoded form would be larger
// than the encoded bytes, and accepts canonical encodings, including static
// composites placed before dynamic arguments.
func TestSetupABIDecodedSizeLimit(t *testing.T) {
	abiJSON := `[{"inputs":[{"internalType":"address","name":"depositor","type":"address"},{"components":[{"internalType":"string","name":"denom","type":"string"},{"internalType":"uint256","name":"amount","type":"uint256"}],"internalType":"struct Coin[]","name":"amount","type":"tuple[]"}],"name":"fundCommunityPool","outputs":[],"stateMutability":"nonpayable","type":"function"},` +
		`{"inputs":[{"internalType":"string","name":"sourcePort","type":"string"},{"internalType":"string","name":"sourceChannel","type":"string"},{"internalType":"string","name":"denom","type":"string"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"address","name":"sender","type":"address"},{"internalType":"string","name":"receiver","type":"string"},{"components":[{"internalType":"uint64","name":"revisionNumber","type":"uint64"},{"internalType":"uint64","name":"revisionHeight","type":"uint64"}],"internalType":"struct Height","name":"timeoutHeight","type":"tuple"},{"internalType":"uint64","name":"timeoutTimestamp","type":"uint64"},{"internalType":"string","name":"memo","type":"string"}],"name":"transfer","outputs":[{"internalType":"uint64","name":"nextSequence","type":"uint64"}],"stateMutability":"nonpayable","type":"function"},` +
		`{"inputs":[{"internalType":"uint256[2]","name":"values","type":"uint256[2]"},{"components":[{"internalType":"uint256","name":"a","type":"uint256"},{"internalType":"uint256[2]","name":"b","type":"uint256[2]"}],"internalType":"struct S","name":"s","type":"tuple"},{"internalType":"string","name":"name","type":"string"},{"internalType":"bytes[]","name":"proofs","type":"bytes[]"}],"name":"fixed","outputs":[],"stateMutability":"nonpayable","type":"function"},` +
		`{"inputs":[{"internalType":"string[][]","name":"values","type":"string[][]"}],"name":"nested","outputs":[],"stateMutability":"nonpayable","type":"function"}]`
	testABI, err := abi.JSON(strings.NewReader(abiJSON))
	require.NoError(t, err)
	isTx := func(*abi.Method) bool { return false }

	pack := func(method string, args ...interface{}) []byte {
		packed, err := testABI.Methods[method].Inputs.Pack(args...)
		require.NoError(t, err)
		return packed
	}
	word := func(v int64) []byte {
		return ethmath.U256Bytes(big.NewInt(v))
	}

	type coin struct {
		Denom  string
		Amount *big.Int
	}
	depositor := common.HexToAddress("0x000000000000000000000000000000000000dEaD")
	oneEther, ok := new(big.Int).SetString("1000000000000000000", 10)
	require.True(t, ok)

	const (
		numElements = 1000
		strLength   = 400
		paddedLen   = ((strLength + 31) / 32) * 32
		numOuter    = 200
		numInner    = 200
	)

	// Every tuple head references the same tuple, whose string head references
	// a shared tail: encoded length is O(N + M) while the decoded form is O(N * M).
	var aliasedCoins []byte
	aliasedCoins = append(aliasedCoins, common.LeftPadBytes(depositor.Bytes(), 32)...)
	aliasedCoins = append(aliasedCoins, word(64)...)          // coins offset
	aliasedCoins = append(aliasedCoins, word(numElements)...) // array length
	for i := 0; i < numElements; i++ {
		aliasedCoins = append(aliasedCoins, word(numElements*32)...) // tuple relative to element heads
	}
	aliasedCoins = append(aliasedCoins, word(64)...)                // string offset within tuple
	aliasedCoins = append(aliasedCoins, word(1)...)                 // amount
	aliasedCoins = append(aliasedCoins, word(strLength)...)         // string length
	aliasedCoins = append(aliasedCoins, make([]byte, paddedLen)...) // padded string content

	// The same shape with distinct tails.
	var distinctCoins []byte
	distinctCoins = append(distinctCoins, common.LeftPadBytes(depositor.Bytes(), 32)...)
	distinctCoins = append(distinctCoins, word(64)...)
	distinctCoins = append(distinctCoins, word(numElements)...)
	elemsHeadPos := len(distinctCoins)
	distinctCoins = append(distinctCoins, make([]byte, numElements*32)...)
	for i := 0; i < numElements; i++ {
		copy(distinctCoins[elemsHeadPos+32*i:], word(int64(len(distinctCoins)-elemsHeadPos)))
		distinctCoins = append(distinctCoins, word(64)...)
		distinctCoins = append(distinctCoins, word(1)...)
		distinctCoins = append(distinctCoins, word(strLength)...)
		distinctCoins = append(distinctCoins, make([]byte, paddedLen)...)
	}

	// N outer string[] offsets alias one inner array of M offsets that all
	// point to the same empty string: encoded in O(N + M) bytes but decoded
	// into N separate M-element slices.
	var aliasedNested []byte
	aliasedNested = append(aliasedNested, word(32)...)       // outer slice offset
	aliasedNested = append(aliasedNested, word(numOuter)...) // outer slice length
	for i := 0; i < numOuter; i++ {
		aliasedNested = append(aliasedNested, word(numOuter*32)...) // inner slice right after the heads
	}
	aliasedNested = append(aliasedNested, word(numInner)...) // inner slice length
	for i := 0; i < numInner; i++ {
		aliasedNested = append(aliasedNested, word(numInner*32)...) // empty string right after the heads
	}
	aliasedNested = append(aliasedNested, word(0)...) // empty string length

	testCases := []struct {
		name   string
		method string
		input  []byte
		expErr string
		check  func(t *testing.T, args []interface{})
	}{
		{
			name:   "canonical tuple array",
			method: "fundCommunityPool",
			input:  pack("fundCommunityPool", depositor, []coin{{Denom: strings.Repeat("a", 100), Amount: big.NewInt(1)}}),
		},
		{
			name:   "static tuple fields larger than the input are not read as offsets",
			method: "fundCommunityPool",
			input:  pack("fundCommunityPool", depositor, []coin{{Denom: "stake", Amount: big.NewInt(1000)}, {Denom: "aatom", Amount: oneEther}}),
			check: func(t *testing.T, args []interface{}) {
				t.Helper()
				require.Len(t, args, 2)
			},
		},
		{
			name:   "tuple array heads aliasing one tail",
			method: "fundCommunityPool",
			input:  aliasedCoins,
			expErr: "cumulative decoded size exceeds encoded input size",
		},
		{
			name:   "tuple array heads with distinct tails",
			method: "fundCommunityPool",
			input:  distinctCoins,
		},
		{
			// the two-word timeoutHeight tuple puts the memo offset at head word 9
			name:   "static tuple before dynamic arguments",
			method: "transfer",
			input: pack("transfer",
				"transfer", "channel-0", "aatom", oneEther, depositor, "cosmos1receiver",
				struct {
					RevisionNumber uint64
					RevisionHeight uint64
				}{RevisionNumber: 1, RevisionHeight: 1_000_000},
				uint64(1_700_000_000_000_000_000),
				"memo",
			),
			check: func(t *testing.T, args []interface{}) {
				t.Helper()
				require.Equal(t, "memo", args[8])
			},
		},
		{
			name:   "fixed arrays before dynamic arguments",
			method: "fixed",
			input: pack("fixed",
				[2]*big.Int{oneEther, oneEther},
				struct {
					A *big.Int
					B [2]*big.Int
				}{A: oneEther, B: [2]*big.Int{oneEther, oneEther}},
				"name",
				[][]byte{[]byte("proof-a"), []byte("proof-b")},
			),
			check: func(t *testing.T, args []interface{}) {
				t.Helper()
				require.Equal(t, "name", args[2])
				require.Equal(t, [][]byte{[]byte("proof-a"), []byte("proof-b")}, args[3])
			},
		},
		{
			name:   "canonical nested string arrays",
			method: "nested",
			input:  pack("nested", [][]string{{"a", ""}, {}, {"b"}}),
		},
		{
			name:   "nested string arrays aliasing one inner array",
			method: "nested",
			input:  aliasedNested,
			expErr: "cumulative decoded size exceeds encoded input size",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := append(append([]byte{}, testABI.Methods[tc.method].ID...), tc.input...)
			method, args, err := SetupABI(testABI, &vm.Contract{Input: input}, false, isTx)
			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.method, method.Name)
			if tc.check != nil {
				tc.check(t, args)
			}
		})
	}
}
