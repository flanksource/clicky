package formatters

import (
	"context"
	"github.com/charmbracelet/x/ansi"

	rpchttp "github.com/flanksource/clicky/rpc/http"
)

// FormatWithContext formats data with an optional caller context that is
// forwarded to registered format callbacks.
func (f *FormatManager) FormatWithContext(ctx any, options FormatOptions, data ...any) (string, error) {
	defer rpchttp.Track(callerContext(ctx), "format")()
	options.NoColor = options.NoColor || f.noColor
	before := collapseFormatInput(data...)
	before = f.applyBeforeFormatCallbacks(ctx, options, before)
	output, err := f.formatWithOptions(options, expandFormatInput(before)...)
	if err != nil {
		return "", err
	}
	output = f.applyAfterFormatCallbacks(ctx, options, before, output)
	if options.NoColor {
		output = ansi.Strip(output)
	}
	return output, nil
}

func callerContext(value any) context.Context {
	if ctx, ok := value.(context.Context); ok {
		return ctx
	}
	if provider, ok := value.(interface{ Context() context.Context }); ok {
		return provider.Context()
	}
	return context.Background()
}
