package aichat

import (
	"context"
	"maps"
	"sort"
	"sync"

	"github.com/firebase/genkit/go/ai"
)

const anthropicMaxStrictTools = 20

type warningFunc func(string, ...any)

// anthropicStrictToolsMiddleware makes strict schemas opt-in for every enabled
// tool. Genkit's Anthropic adapter otherwise treats missing strict metadata as
// true. Explicit opt-ins remain strict up to Anthropic's 20-tool limit; every
// other tool is sent non-strict without being removed.
func anthropicStrictToolsMiddleware(tools []registeredTool, warnf warningFunc) ai.ModelMiddleware {
	candidates := strictToolCandidates(tools)
	if len(tools) == 0 {
		return nil
	}

	strictCount := min(len(candidates), anthropicMaxStrictTools)
	strictNames := make(map[string]bool, strictCount)
	for _, tool := range candidates[:strictCount] {
		strictNames[tool.info.Name] = true
	}
	knownNames := make(map[string]bool, len(tools))
	for _, tool := range tools {
		knownNames[tool.info.Name] = true
	}

	var warningOnce sync.Once
	return func(next ai.ModelFunc) ai.ModelFunc {
		return func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
			if len(candidates) > anthropicMaxStrictTools {
				warningOnce.Do(func() {
					if warnf != nil {
						warnf("Anthropic supports at most %d strict tools; keeping %d of %d explicit opt-ins strict and sending %d as non-strict", anthropicMaxStrictTools, anthropicMaxStrictTools, len(candidates), len(candidates)-anthropicMaxStrictTools)
					}
				})
			}

			clonedReq := *req
			clonedReq.Tools = make([]*ai.ToolDefinition, len(req.Tools))
			for i, tool := range req.Tools {
				if tool == nil || !knownNames[tool.Name] {
					clonedReq.Tools[i] = tool
					continue
				}
				clonedTool := *tool
				clonedTool.Metadata = maps.Clone(tool.Metadata)
				if clonedTool.Metadata == nil {
					clonedTool.Metadata = map[string]any{}
				}
				clonedTool.Metadata["strict"] = strictNames[tool.Name]
				clonedReq.Tools[i] = &clonedTool
			}
			return next(ctx, &clonedReq, cb)
		}
	}
}

func strictToolCandidates(tools []registeredTool) []registeredTool {
	candidates := make([]registeredTool, 0, len(tools))
	for _, tool := range tools {
		if explicitlyStrict(tool.info) {
			candidates = append(candidates, tool)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i].info, candidates[j].info
		if leftRisk, rightRisk := strictRiskRank(left), strictRiskRank(right); leftRisk != rightRisk {
			return leftRisk < rightRisk
		}
		return left.Name < right.Name
	})
	return candidates
}

func explicitlyStrict(tool ToolInfo) bool {
	return tool.Strict != nil && *tool.Strict
}

// Lower ranks retain strict validation first: destructive, non-idempotent,
// mutating, unknown, then confirmed read-only tools.
func strictRiskRank(tool ToolInfo) int {
	switch {
	case tool.DestructiveHint != nil && *tool.DestructiveHint:
		return 0
	case tool.IdempotentHint != nil && !*tool.IdempotentHint:
		return 1
	case tool.ReadOnlyHint != nil && !*tool.ReadOnlyHint:
		return 2
	case tool.ReadOnlyHint == nil:
		return 3
	default:
		return 4
	}
}
