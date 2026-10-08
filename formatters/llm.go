package formatters

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/flanksource/clicky/api"
)

// LLMPageInfo carries the transport's paging facts and continuation.
type LLMPageInfo struct {
	Offset      int
	Total       *int64
	TotalExact  bool
	HasMore     *bool
	Next        string
	Pageable    bool
	Cursor      bool
	moreUnknown bool
}

func formatLLM(data any, options FormatOptions) (string, error) {
	page := LLMPageInfo{}
	pageLimit := 0
	if options.LLMPage != nil {
		page = *options.LLMPage
	}
	if paged, ok := data.(api.Paged); ok {
		meta := paged.PageMetadata()
		page.Offset = meta.Offset
		pageLimit = meta.Limit
		page.Pageable = true
		if meta.Total >= 0 {
			page.Total = &meta.Total
			page.TotalExact = true
		}
		data = paged.PageRows()
	}
	table, available, err := llmTable(data)
	if err != nil {
		return "", err
	}
	if table == nil {
		return NewMarkdownFormatter().FormatWithOptions(data, FormatOptions{NoColor: true})
	}
	shown := min(available, api.LLMPageSize)
	table.Rows = table.Rows[:shown]
	table.RowDetail = nil
	more := available > shown
	if page.Total != nil && page.TotalExact {
		more = more || int64(page.Offset+shown) < *page.Total
	}
	if page.HasMore != nil {
		more = available > shown || *page.HasMore
	}
	if !more && page.HasMore == nil && page.Total == nil && pageLimit > 0 && available >= pageLimit {
		page.moreUnknown = true
		more = true
	}
	return api.RenderMarkdown(*table, api.MarkdownOptions{NoColor: true}) + LLMPageFooter(page, shown, more), nil
}

func LLMPageFooter(page LLMPageInfo, shown int, more bool) string {
	total := "unknown"
	if page.Total != nil {
		total = strconv.FormatInt(*page.Total, 10)
		if !page.TotalExact {
			total += " (estimate)"
		}
	}
	rangeText := "empty"
	if shown > 0 {
		rangeText = fmt.Sprintf("%d–%d", page.Offset+1, page.Offset+shown)
	}
	if page.Cursor {
		rangeText = "current page"
	}
	hasMore := strconv.FormatBool(more)
	if page.moreUnknown {
		hasMore = "unknown"
	}
	out := fmt.Sprintf("\nShown: %d (rows %s); Total: %s; Has more: %s.\n", shown, rangeText, total, hasMore)
	if more && shown > 0 {
		if !page.Pageable {
			out += "Paging is unavailable for this operation. Use a full export to read the remaining rows.\n"
		} else {
			next := page.Next
			if next == "" {
				next = fmt.Sprintf("--offset %d --limit %d", page.Offset+shown, api.LLMPageSize)
			}
			out += "Next: " + next + ". Keep the same filters, sort and entity scope.\n"
		}
	}
	return out + "Use --json for complete fields and all returned rows.\n"
}

func llmTable(data any) (*api.TextTable, int, error) {
	switch value := data.(type) {
	case api.TextTable:
		return &value, len(value.Rows), nil
	case *api.TextTable:
		if value == nil {
			return nil, 0, fmt.Errorf("nil LLM table")
		}
		copy := *value
		return &copy, len(copy.Rows), nil
	case *api.PrettyData:
		if value == nil {
			return nil, 0, fmt.Errorf("nil LLM data")
		}
		if value.Table != nil {
			return llmTable(value.Table)
		}
		return llmTable(value.Original)
	}
	value := reflect.ValueOf(data)
	if !value.IsValid() || (value.Kind() != reflect.Slice && value.Kind() != reflect.Array) {
		return nil, 0, nil
	}
	available := value.Len()
	if available == 0 {
		element := value.Type().Elem()
		zero := reflect.Zero(element)
		if element.Kind() == reflect.Pointer {
			zero = reflect.New(element.Elem())
		}
		if zero.IsValid() {
			if provider, ok := zero.Interface().(api.TableProvider); ok {
				table := api.NewEmptyTable(provider.Columns())
				return &table, 0, nil
			}
		}
	}
	providers := make([]api.TableProvider, 0, min(available, api.LLMPageSize))
	for i := 0; i < min(available, api.LLMPageSize); i++ {
		row := value.Index(i)
		if (row.Kind() == reflect.Pointer || row.Kind() == reflect.Interface) && row.IsNil() {
			return nil, 0, fmt.Errorf("nil LLM row at index %d", i)
		}
		provider, ok := row.Interface().(api.TableProvider)
		if !ok {
			return reflectedLLMTable(value, available)
		}
		providers = append(providers, provider)
	}
	table := api.NewTableFrom(providers, api.TableOptions{SkipDetails: true})
	return &table, available, nil
}

func reflectedLLMTable(value reflect.Value, available int) (*api.TextTable, int, error) {
	rows := reflect.MakeSlice(reflect.SliceOf(value.Type().Elem()), min(available, api.LLMPageSize), min(available, api.LLMPageSize))
	reflect.Copy(rows, value)
	data, err := ToPrettyDataWithOptions(rows.Interface(), FormatOptions{Table: true})
	if err != nil {
		return nil, 0, err
	}
	return data.Table, available, nil
}
