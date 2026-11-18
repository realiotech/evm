package gov

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	evmaddress "github.com/cosmos/evm/encoding/address"
	cmn "github.com/cosmos/evm/precompiles/common"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authz "github.com/cosmos/cosmos-sdk/x/authz"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestNewMsgDeposit(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	depositorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	validCoins := []cmn.Coin{{Denom: "stake", Amount: big.NewInt(1000)}}
	proposalID := uint64(1)

	expectedDepositorAddr, err := addrCodec.BytesToString(depositorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name           string
		args           []interface{}
		wantErr        bool
		errMsg         string
		wantDepositor  string
		wantProposalID uint64
	}{
		{
			name:           "valid",
			args:           []interface{}{depositorAddr, proposalID, validCoins},
			wantErr:        false,
			wantDepositor:  expectedDepositorAddr,
			wantProposalID: proposalID,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 3, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{depositorAddr, proposalID, validCoins, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 3, 4),
		},
		{
			name:    "invalid depositor type",
			args:    []interface{}{"not-an-address", proposalID, validCoins},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidDepositor, "not-an-address"),
		},
		{
			name:    "empty depositor address",
			args:    []interface{}{common.Address{}, proposalID, validCoins},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidDepositor, common.Address{}),
		},
		{
			name:    "invalid proposal ID type",
			args:    []interface{}{depositorAddr, "not-a-uint64", validCoins},
			wantErr: true,
			errMsg:  "invalid proposal id",
		},
		{
			name:    "invalid coins",
			args:    []interface{}{depositorAddr, proposalID, "invalid-coins"},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidDeposits, "deposit arg"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgDeposit(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, msg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				require.Equal(t, depositorAddr, returnAddr)
				require.Equal(t, tt.wantDepositor, msg.Depositor)
				require.Equal(t, tt.wantProposalID, msg.ProposalId)
				require.NotEmpty(t, msg.Amount)
			}
		})
	}
}

func TestNewMsgCancelProposal(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	proposerAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	proposalID := uint64(1)

	expectedProposerAddr, err := addrCodec.BytesToString(proposerAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name           string
		args           []interface{}
		wantErr        bool
		errMsg         string
		wantProposer   string
		wantProposalID uint64
	}{
		{
			name:           "valid",
			args:           []interface{}{proposerAddr, proposalID},
			wantErr:        false,
			wantProposer:   expectedProposerAddr,
			wantProposalID: proposalID,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 1, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{proposerAddr, proposalID, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 1, 3),
		},
		{
			name:    "invalid proposer type",
			args:    []interface{}{"not-an-address", proposalID},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidProposer, "not-an-address"),
		},
		{
			name:    "empty proposer address",
			args:    []interface{}{common.Address{}, proposalID},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidProposer, common.Address{}),
		},
		{
			name:    "invalid proposal ID type",
			args:    []interface{}{proposerAddr, "not-a-uint64"},
			wantErr: true,
			errMsg:  "invalid proposal id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgCancelProposal(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, msg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				require.Equal(t, proposerAddr, returnAddr)
				require.Equal(t, tt.wantProposer, msg.Proposer)
				require.Equal(t, tt.wantProposalID, msg.ProposalId)
			}
		})
	}
}

func TestNewMsgVote(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	voterAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	proposalID := uint64(1)
	option := uint8(1)
	metadata := "test-metadata"

	expectedVoterAddr, err := addrCodec.BytesToString(voterAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name           string
		args           []interface{}
		wantErr        bool
		errMsg         string
		wantVoter      string
		wantProposalID uint64
		wantOption     uint8
		wantMetadata   string
	}{
		{
			name:           "valid",
			args:           []interface{}{voterAddr, proposalID, option, metadata},
			wantErr:        false,
			wantVoter:      expectedVoterAddr,
			wantProposalID: proposalID,
			wantOption:     option,
			wantMetadata:   metadata,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 4, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{voterAddr, proposalID, option, metadata, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 4, 5),
		},
		{
			name:    "invalid voter type",
			args:    []interface{}{"not-an-address", proposalID, option, metadata},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidVoter, "not-an-address"),
		},
		{
			name:    "empty voter address",
			args:    []interface{}{common.Address{}, proposalID, option, metadata},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidVoter, common.Address{}),
		},
		{
			name:    "invalid proposal ID type",
			args:    []interface{}{voterAddr, "not-a-uint64", option, metadata},
			wantErr: true,
			errMsg:  "invalid proposal id",
		},
		{
			name:    "invalid option type",
			args:    []interface{}{voterAddr, proposalID, "not-a-uint8", metadata},
			wantErr: true,
			errMsg:  "invalid option",
		},
		{
			name:    "invalid metadata type",
			args:    []interface{}{voterAddr, proposalID, option, 123},
			wantErr: true,
			errMsg:  "invalid metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgVote(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, msg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				require.Equal(t, voterAddr, returnAddr)
				require.Equal(t, tt.wantVoter, msg.Voter)
				require.Equal(t, tt.wantProposalID, msg.ProposalId)
				require.Equal(t, tt.wantOption, uint8(msg.Option)) //nolint:gosec // doesn't matter here
				require.Equal(t, tt.wantMetadata, msg.Metadata)
			}
		})
	}
}

func TestParseVoteArgs(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	voterAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	proposalID := uint64(1)

	expectedVoterAddr, err := addrCodec.BytesToString(voterAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name           string
		args           []interface{}
		wantErr        bool
		errMsg         string
		wantVoter      string
		wantProposalID uint64
	}{
		{
			name:           "valid",
			args:           []interface{}{proposalID, voterAddr},
			wantErr:        false,
			wantVoter:      expectedVoterAddr,
			wantProposalID: proposalID,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{proposalID, voterAddr, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 3),
		},
		{
			name:    "invalid proposal ID type",
			args:    []interface{}{"not-a-uint64", voterAddr},
			wantErr: true,
			errMsg:  "invalid proposal id",
		},
		{
			name:    "invalid voter type",
			args:    []interface{}{proposalID, "not-an-address"},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidVoter, "not-an-address"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := ParseVoteArgs(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, req)
			} else {
				require.NoError(t, err)
				require.NotNil(t, req)
				require.Equal(t, tt.wantVoter, req.Voter)
				require.Equal(t, tt.wantProposalID, req.ProposalId)
			}
		})
	}
}

func TestParseDepositArgs(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	depositorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	proposalID := uint64(1)

	expectedDepositorAddr, err := addrCodec.BytesToString(depositorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name           string
		args           []interface{}
		wantErr        bool
		errMsg         string
		wantDepositor  string
		wantProposalID uint64
	}{
		{
			name:           "valid",
			args:           []interface{}{proposalID, depositorAddr},
			wantErr:        false,
			wantDepositor:  expectedDepositorAddr,
			wantProposalID: proposalID,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{proposalID, depositorAddr, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 3),
		},
		{
			name:    "invalid proposal ID type",
			args:    []interface{}{"not-a-uint64", depositorAddr},
			wantErr: true,
			errMsg:  "invalid proposal id",
		},
		{
			name:    "invalid depositor type",
			args:    []interface{}{proposalID, "not-an-address"},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidDepositor, "not-an-address"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := ParseDepositArgs(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, req)
			} else {
				require.NoError(t, err)
				require.NotNil(t, req)
				require.Equal(t, tt.wantDepositor, req.Depositor)
				require.Equal(t, tt.wantProposalID, req.ProposalId)
			}
		})
	}
}

func TestNewMsgSubmitProposalRequiresCanonicalMessageJSON(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	registry := codectypes.NewInterfaceRegistry()
	authz.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	proposerAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	deposit := []cmn.Coin{{Denom: "stake", Amount: big.NewInt(1000)}}

	grant := func(authorization, expiration string) string {
		return `{"@type":"/cosmos.authz.v1beta1.MsgGrant",` +
			`"granter":"cosmos10d07y265gmmuvt4z0w9aw880jnsr700j6zn9kn",` +
			`"grantee":"cosmos1yg3zyg3zyg3zyg3zyg3zyg3zyg3zyg3zwqjy6c",` +
			`"grant":{"authorization":{"@type":"/cosmos.staking.v1beta1.StakeAuthorization",` + authorization + `},` +
			`"expiration":"` + expiration + `"}}`
	}
	exec := func(msg string) string {
		return `{"@type":"/cosmos.authz.v1beta1.MsgExec","grantee":"cosmos10d07y265gmmuvt4z0w9aw880jnsr700j6zn9kn","msgs":[` + msg + `]}`
	}

	const (
		allowList  = `"allow_list":{"address":["cosmosvaloper1xvenxvenxvenxvenxvenxvenxvenxvene23vre"]}`
		denyList   = `"deny_list":{"address":["cosmosvaloper1g3zyg3zyg3zyg3zyg3zyg3zyg3zyg3zyx9en9h"]}`
		delegate   = `"authorization_type":"AUTHORIZATION_TYPE_DELEGATE"`
		expiration = "2030-01-01T00:00:00Z"
	)
	canonicalGrant := grant(`"max_tokens":null,`+allowList+`,`+delegate, expiration)

	for name, msg := range map[string]string{
		"canonical grant":         canonicalGrant,
		"canonical grant in exec": exec(canonicalGrant),
	} {
		t.Run(name, func(t *testing.T) {
			res, _, err := NewMsgSubmitProposal(
				[]interface{}{proposerAddr, proposalJSON(msg), deposit},
				cdc, addrCodec,
			)
			require.NoError(t, err)
			require.Len(t, res.Messages, 1)
		})
	}

	ambiguousGrant := grant(`"max_tokens":null,`+allowList+`,`+denyList+`,`+delegate, expiration)
	for name, msg := range map[string]string{
		"both oneof members":         ambiguousGrant,
		"both oneof members in exec": exec(ambiguousGrant),
		"camelCase oneof member": grant(
			`"max_tokens":null,"allowList":{"address":["cosmosvaloper1xvenxvenxvenxvenxvenxvenxvenxvene23vre"]},`+denyList+`,`+delegate,
			expiration,
		),
		"null oneof member":  grant(`"max_tokens":null,"allow_list":null,`+denyList+`,`+delegate, expiration),
		"numeric enum":       grant(`"max_tokens":null,`+allowList+`,"authorization_type":1`, expiration),
		"non-UTC timestamp":  grant(`"max_tokens":null,`+allowList+`,`+delegate, "2030-01-01T01:00:00+01:00"),
		"omitted default":    grant(allowList+`,`+delegate, expiration),
		"extra whitespace":   strings.Replace(canonicalGrant, `,`, `, `, 1),
		"undecodable member": grant(`"max_tokens":null,"allow_list":{"unknown_allow":1},"deny_list":{"unknown_deny":1},`+delegate, expiration),
	} {
		t.Run(name, func(t *testing.T) {
			// gogoproto's choice among several set oneof members varies between
			// runs; the result must not.
			for i := 0; i < 256; i++ {
				_, _, err := NewMsgSubmitProposal(
					[]interface{}{proposerAddr, proposalJSON(msg), deposit},
					cdc, addrCodec,
				)
				require.ErrorIs(t, err, errInvalidProposalMessage)
				require.Equal(t, "message 0: "+errInvalidProposalMessage.Error(), err.Error())
			}
		})
	}
}

func proposalJSON(msg string) []byte {
	return []byte(fmt.Sprintf(
		`{"title":"t","summary":"s","metadata":"","expedited":false,"messages":[%s]}`, msg,
	))
}
