// Media declarations: what an operation exchanges when it is not the JSON
// clicky assumes, so uploads and downloads describe themselves.
package entity

import (
	"sync"

	"github.com/spf13/cobra"
)

// MediaSpec describes a request or response body that is not JSON.
//
// It exists so an operation that takes a file upload or returns a bundle is
// described correctly in the generated OpenAPI document, rather than being
// documented as the JSON body clicky would otherwise assume and then corrected
// by hand afterwards.
//
// Schema may be left zero for an opaque payload — a zip, a PDF, a spreadsheet —
// which is documented as a binary string.
type MediaSpec struct {
	// ContentType is the media type exchanged, e.g. "multipart/form-data" or
	// "application/zip".
	ContentType string
	// Description explains the payload to whoever reads the document.
	Description string
	// Schema describes the payload's shape when it has one.
	Schema Schema
	// Required marks a request body the operation cannot run without. It has no
	// meaning on a response.
	Required bool
}

// CommandMedia is what one command declared about its non-JSON bodies. Either
// side may be nil: an upload that answers in JSON declares only a request, a
// download that takes query parameters only a response.
type CommandMedia struct {
	Request  *MediaSpec
	Response *MediaSpec
}

// mediaRegistry maps cobra commands to their media declarations. The RPC
// converter reads it the same way it reads the data and lookup registries.
var mediaRegistry sync.Map // map[*cobra.Command]CommandMedia

// AnnotateMedia declares what a command exchanges when it is not JSON.
//
// It describes the operation; it does not change how it runs. A handler reading
// a multipart upload still does so from the request on its context
// (rpc.RequestFromContext), because clicky has no typed parameter that could
// carry a file.
func AnnotateMedia(cmd *cobra.Command, media CommandMedia) {
	if cmd == nil {
		return
	}
	mediaRegistry.Store(cmd, media)
}

// GetCommandMedia returns what a command declared, or a zero value.
func GetCommandMedia(cmd *cobra.Command) CommandMedia {
	if v, ok := mediaRegistry.Load(cmd); ok {
		return v.(CommandMedia)
	}
	return CommandMedia{}
}
