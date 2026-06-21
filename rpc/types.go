package rpc

import "github.com/flanksource/clicky/entity"

// The RPC operation model's canonical home is the entity/ subpackage (see the
// inversion described in entity/doc.go). These aliases keep rpc.RPCOperation,
// rpc.Schema, etc. working for existing callers (rpc internals, mcp, aichat).
type (
	DataFunc            = entity.DataFunc
	ContextDataFunc     = entity.ContextDataFunc
	ContextLookupFunc   = entity.ContextLookupFunc
	RPCOperation        = entity.RPCOperation
	ClickySpecMeta      = entity.ClickySpecMeta
	ClickySurface       = entity.ClickySurface
	ClickyOperationMeta = entity.ClickyOperationMeta
	RPCParameter        = entity.RPCParameter
	RPCService          = entity.RPCService
	Schema              = entity.Schema
	Property            = entity.Property
	Config              = entity.Config
)

// DefaultConfig returns a sensible default RPC conversion configuration.
var DefaultConfig = entity.DefaultConfig
