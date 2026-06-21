package clicky

import (
	"reflect"
	"sync"

	"github.com/spf13/cobra"
)

// ResponseOpenAPIMeta describes the static response type for a generated
// command. It is consumed by the RPC/OpenAPI layer without executing handlers.
type ResponseOpenAPIMeta struct {
	Type     reflect.Type
	Array    bool
	Paged    bool
	EntityID bool
}

var responseMetaRegistry sync.Map // map[*cobra.Command]ResponseOpenAPIMeta

// SetCommandResponseMeta attaches static response metadata to a command.
func SetCommandResponseMeta(cmd *cobra.Command, meta ResponseOpenAPIMeta) {
	if cmd == nil || meta.Type == nil {
		return
	}
	responseMetaRegistry.Store(cmd, meta)
}

// GetCommandResponseMeta returns static response metadata attached to a command.
func GetCommandResponseMeta(cmd *cobra.Command) *ResponseOpenAPIMeta {
	if v, ok := responseMetaRegistry.Load(cmd); ok {
		meta := v.(ResponseOpenAPIMeta)
		return &meta
	}
	return nil
}

func responseTypeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}
