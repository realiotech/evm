package cosmos

import (
	"slices"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ baseapp.MessageRouter = DenyListMsgRouter{}

// DenyListMsgRouter wraps a message router and returns no handler for the
// denied msg types.
//
// # When to use it
//
// Use it when a msg is only safe to execute after a check that runs in the
// ante handler and not in the msg server. The ante handler only runs on the
// msgs of a top-level tx. A module that dispatches msgs itself looks the
// handler up in the message router it was constructed with and calls the msg
// server directly, so the ante checks never run for the msgs it dispatches.
// Modules that dispatch msgs include authz (MsgExec), gov (proposals), group
// (group policy proposals), the ICA host (interchain account txs) and contract
// modules such as wasm.
//
// A chain needs this router when both of these hold:
//
//   - its ante handler enforces a security rule on a msg type, such as who may
//     send it or what it must pay, that the msg server does not enforce again;
//   - it wires in a module that can dispatch that msg type.
//
// # How to use it
//
// Pass the router, in place of app.MsgServiceRouter(), to the keeper of every
// module that dispatches msgs, with the type URL of each msg to block:
//
//	deniedMsgRouter := cosmosante.NewDenyListMsgRouter(
//		app.MsgServiceRouter(),
//		sdk.MsgTypeURL(&customtypes.MsgCheckedByAnteHandler{}),
//	)
//	app.AuthzKeeper = authzkeeper.NewKeeper(storeService, appCodec, deniedMsgRouter, app.AccountKeeper)
//
// These keepers then treat a denied msg as a msg without a route: authz
// refuses to grant or execute it, gov refuses the proposal at submission,
// group accepts the proposal but fails its execution, and the ICA host fails
// the packet.
//
// # Limits
//
// The router only covers the keepers it is passed to. A dispatching module
// that is constructed with app.MsgServiceRouter() still reaches the msg
// server, and top-level txs are not affected. Where possible, enforce the rule
// in the msg server instead, which covers every dispatch path at once.
//
// MsgEthereumTx does not need to be denied: its msg server only executes a msg
// that the EVM ante handler verified in the same tx.
type DenyListMsgRouter struct {
	router         baseapp.MessageRouter
	deniedMsgTypes []string
}

// NewDenyListMsgRouter creates a message router that routes msgs to the given
// router, except for the denied msg types.
func NewDenyListMsgRouter(router baseapp.MessageRouter, deniedMsgTypes ...string) DenyListMsgRouter {
	return DenyListMsgRouter{
		router:         router,
		deniedMsgTypes: deniedMsgTypes,
	}
}

// Handler returns the handler of the given msg, or nil if its type is denied.
func (r DenyListMsgRouter) Handler(msg sdk.Msg) baseapp.MsgServiceHandler {
	if r.isDenied(sdk.MsgTypeURL(msg)) {
		return nil
	}
	return r.router.Handler(msg)
}

// HandlerByTypeURL returns the handler of the given msg type url, or nil if it
// is denied.
func (r DenyListMsgRouter) HandlerByTypeURL(typeURL string) baseapp.MsgServiceHandler {
	if r.isDenied(typeURL) {
		return nil
	}
	return r.router.HandlerByTypeURL(typeURL)
}

func (r DenyListMsgRouter) isDenied(typeURL string) bool {
	return slices.Contains(r.deniedMsgTypes, typeURL)
}
