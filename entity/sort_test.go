package entity

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/flanksource/clicky/api"
)

type sortableEntity struct {
	ID         string    `json:"id"`
	Name       string    `json:"name" sort:"name"`
	UpdatedGMT time.Time `json:"updatedGMT" pretty:"label=Updated" sort:"updated"`
}

func (s sortableEntity) GetID() string   { return s.ID }
func (s sortableEntity) GetName() string { return s.Name }
func (sortableEntity) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		api.Column("name").Build(),
		api.Column("updatedGMT").Label("Updated").Build(),
		api.Column("id").Build(),
	}
}
func (s sortableEntity) Row() map[string]any {
	return map[string]any{"name": s.Name, "updatedGMT": s.UpdatedGMT, "id": s.ID}
}

type sortableEntityOptions struct {
	SortOptions
}

type nonSortableEntityOptions struct{}

func TestEntityListSortResolution(t *testing.T) {
	tests := []struct {
		name      string
		flags     map[string]string
		want      SortOptions
		wantError string
	}{
		{
			name: "uses entity default when no sort parameters are supplied",
			want: SortOptions{Key: "updated", Direction: SortDirectionDesc},
		},
		{
			name:  "defaults an explicitly selected column to ascending",
			flags: map[string]string{"sort": "name"},
			want:  SortOptions{Key: "name", Direction: SortDirectionAsc},
		},
		{
			name:  "accepts descending direction for an explicitly selected column",
			flags: map[string]string{"sort": "name", "order": "desc"},
			want:  SortOptions{Key: "name", Direction: SortDirectionDesc},
		},
		{
			name:      "rejects direction without a selected column",
			flags:     map[string]string{"order": "asc"},
			wantError: "order requires sort",
		},
		{
			name:      "rejects a column not declared by the response struct",
			flags:     map[string]string{"sort": "id"},
			wantError: "unsupported sort key",
		},
		{
			name:      "rejects an invalid direction",
			flags:     map[string]string{"sort": "name", "order": "sideways"},
			wantError: "unsupported sort direction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured sortableEntityOptions
			op, ok := entityListOperation(Entity[sortableEntity, sortableEntityOptions, sortableEntity]{
				Name: "sortable",
				Sort: &SortSpec{Default: SortOptions{Key: "updated", Direction: SortDirectionDesc}},
				List: func(opts sortableEntityOptions) ([]sortableEntity, error) {
					captured = opts
					return nil, nil
				},
			})
			if !ok {
				t.Fatal("expected list operation")
			}

			_, err := op.DataFunc(tt.flags, nil)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
				}
				status, ok := err.(*StatusError)
				if !ok || status.StatusCode() != http.StatusBadRequest || status.Code != "invalid_sort" {
					t.Fatalf("expected invalid_sort status error, got %#v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("list operation failed: %v", err)
			}
			if captured.SortOptions != tt.want {
				t.Fatalf("expected sort %#v, got %#v", tt.want, captured.SortOptions)
			}
		})
	}
}

func TestSortableEntityRequiresSortCarrier(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected sortable entity registration to panic")
		}
	}()

	RegisterEntity(Entity[sortableEntity, nonSortableEntityOptions, sortableEntity]{
		Name: "missing-sort-carrier",
		Sort: &SortSpec{Default: SortOptions{Key: "updated", Direction: SortDirectionDesc}},
		List: func(nonSortableEntityOptions) ([]sortableEntity, error) { return nil, nil },
	})
}

func TestEntityWrapperPreservesStructSortMetadata(t *testing.T) {
	columns := entityWithID[sortableEntity]{Inner: sortableEntity{}}.Columns()
	keys := map[string]string{}
	for _, column := range columns {
		keys[column.Name] = column.SortKey
	}
	if keys["name"] != "name" || keys["updatedGMT"] != "updated" || keys["_id"] != "" || keys["id"] != "" {
		t.Fatalf("unexpected wrapped entity sort keys: %#v", keys)
	}
}
