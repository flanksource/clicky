package rpc

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/flanksource/clicky/formatters"
)

func IsListOperation(op *RPCOperation) bool {
	return op != nil && (op.ResponsePaged || op.PagedFunc != nil || (op.Clicky != nil && op.Clicky.Verb == "list"))
}

func executeLLMPaged(ctx context.Context, op *RPCOperation, flags map[string]string) (string, error) {
	req, err := llmToolPageRequest(flags)
	if err != nil {
		return "", err
	}
	res, err := op.PagedFunc(ctx, req, flags)
	if err != nil {
		return "", err
	}
	if res.Rows == nil {
		return "", fmt.Errorf("operation %s returned no row iterator", op.Name)
	}
	next := ""
	if res.Pageable && res.HasMore {
		if res.Next != "" {
			next = "--cursor " + strconv.Quote(res.Next) + " --limit 25"
		} else {
			next = fmt.Sprintf("--offset %d --limit 25", req.Offset+req.Limit)
		}
	}
	var out strings.Builder
	_, err = formatters.WriteTableStream(ctx, &out, res.Rows, formatters.StreamOptions{Format: "llm", LLMPage: llmPageInfo(req, res, next)})
	return out.String(), err
}
