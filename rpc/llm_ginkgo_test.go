package rpc

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strconv"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/entity"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LLM native paging", func() {
	It("requests 25 rows and resumes at the last displayed row with filters intact", func() {
		server := pagedServer(func(_ context.Context, req entity.PageRequest, flags map[string]string) (entity.PageResponse, error) {
			Expect(req.Limit).To(Equal(25))
			Expect(flags).To(HaveKeyWithValue("account", "assets://example"))
			start := 0
			if req.Cursor != "" {
				var err error
				start, err = strconv.Atoi(req.Cursor)
				Expect(err).NotTo(HaveOccurred())
			}
			rows := make([]map[string]any, req.Limit)
			for i := range rows {
				rows[i] = map[string]any{"n": fmt.Sprintf("item-%03d", start+i)}
			}
			return entity.PageResponse{Rows: &staticRows{columns: []api.ColumnDef{{Name: "n"}}, rows: rows}, Pageable: true, Mode: entity.ModePage, HasMore: true, Next: strconv.Itoa(start + req.Limit)}, nil
		})
		for _, cursor := range []string{"", "25"} {
			rec := httptest.NewRecorder()
			server.handleExecuteCommand(rec, httptest.NewRequest("GET", "/api/v1/config?format=llm&limit=100&account=assets%3A%2F%2Fexample&cursor="+cursor, nil))
			Expect(rec.Code).To(Equal(200))
			Expect(rec.Body.String()).To(ContainSubstring("Shown: 25"))
			Expect(rec.Body.String()).To(ContainSubstring("Use --json for complete fields and all returned rows."))
			Expect(rec.Body.String()).To(ContainSubstring("format=llm"))
			Expect(rec.Body.String()).To(ContainSubstring("account=assets%3A%2F%2Fexample"))
			if cursor == "" {
				Expect(rec.Body.String()).To(ContainSubstring("item-024"))
				Expect(rec.Body.String()).NotTo(ContainSubstring("item-025"))
			} else {
				Expect(rec.Body.String()).To(ContainSubstring("item-025"))
			}
		}
	})
})
