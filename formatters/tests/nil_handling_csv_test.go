package formatters

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/flanksource/clicky/formatters"
)

type zeroImpl struct{ zero bool }

func (z zeroImpl) IsZero() bool { return z.zero }
func (z zeroImpl) String() string {
	if z.zero {
		return "zero:true"
	}
	return "zero:false"
}

type nilImpl struct{ isNil bool }

func (n nilImpl) IsNil() bool { return n.isNil }
func (n nilImpl) String() string {
	if n.isNil {
		return "nil:true"
	}
	return "nil:false"
}

type NilHandlingRow struct {
	Label       string     `json:"label" pretty:"label=Field"`
	UUID        uuid.UUID  `json:"uuid" pretty:"label=uuid.UUID"`
	UUIDPtr     *uuid.UUID `json:"uuid_ptr,omitempty" pretty:"label=*uuid.UUID,omitempty"`
	TimeVal     time.Time  `json:"time_val" pretty:"label=time.Time"`
	TimePtr     *time.Time `json:"time_ptr,omitempty" pretty:"label=*time.Time,omitempty"`
	StringPtr   *string    `json:"string_ptr,omitempty" pretty:"label=*string,omitempty"`
	IntPtr      *int       `json:"int_ptr,omitempty" pretty:"label=*int,omitempty"`
	EmptyStruct zeroImpl   `json:"empty_struct" pretty:"label=IsZero() struct"`
	NilStruct   nilImpl    `json:"nil_struct" pretty:"label=IsNil() struct"`
}

func TestNilHandlingCSV(t *testing.T) {
	fixedTime := time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)
	fixedUUID := uuid.MustParse("37a13269-a180-4b0a-a9f0-40ca87a8a6a3")

	rows := []NilHandlingRow{
		{
			Label:       "All populated",
			UUID:        fixedUUID,
			UUIDPtr:     &fixedUUID,
			TimeVal:     fixedTime,
			TimePtr:     &fixedTime,
			StringPtr:   stringPtr("world"),
			IntPtr:      intPtr(99),
			EmptyStruct: zeroImpl{zero: false},
			NilStruct:   nilImpl{isNil: false},
		},
		{
			Label:       "All zero/nil/empty",
			UUID:        uuid.UUID{},
			UUIDPtr:     nil,
			TimeVal:     time.Time{},
			TimePtr:     nil,
			StringPtr:   nil,
			IntPtr:      nil,
			EmptyStruct: zeroImpl{zero: true},
			NilStruct:   nilImpl{isNil: true},
		},
		{
			Label:       "Mixed",
			UUID:        fixedUUID,
			UUIDPtr:     nil,
			TimeVal:     fixedTime,
			TimePtr:     nil,
			StringPtr:   nil,
			IntPtr:      nil,
			EmptyStruct: zeroImpl{zero: true},
			NilStruct:   nilImpl{isNil: false},
		},
	}

	manager := NewFormatManager()
	csvOutput, err := manager.CSV(rows)
	require.NoError(t, err)
	t.Logf("CSV output:\n%s", csvOutput)

	reader := csv.NewReader(strings.NewReader(csvOutput))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(records), 4, "expected header + 3 data rows")

	headers := records[0]
	colIdx := make(map[string]int)
	for i, h := range headers {
		colIdx[h] = i
	}

	cell := func(row int, col string) string {
		idx, ok := colIdx[col]
		if !ok {
			t.Fatalf("column %q not found in headers: %v", col, headers)
		}
		return records[row][idx]
	}

	uuidStr := "37a13269-a180-4b0a-a9f0-40ca87a8a6a3"

	t.Run("populated row", func(t *testing.T) {
		assert.Equal(t, "All populated", cell(1, "Field"))
		assert.Equal(t, uuidStr, cell(1, "uuid.UUID"))
		assert.Equal(t, uuidStr, cell(1, "*uuid.UUID"))
		assert.NotEmpty(t, cell(1, "time.Time"), "populated time.Time should not be empty")
		assert.NotEqual(t, "{}", cell(1, "time.Time"), "populated time.Time should not be {}")
		assert.NotEmpty(t, cell(1, "*time.Time"))
		assert.Equal(t, "world", cell(1, "*string"))
		assert.Equal(t, "99", cell(1, "*int"))
		assert.Equal(t, "zero:false", cell(1, "IsZero() struct"))
		assert.Equal(t, "nil:false", cell(1, "IsNil() struct"))
	})

	t.Run("all zero/nil/empty row", func(t *testing.T) {
		assert.Equal(t, "All zero/nil/empty", cell(2, "Field"))
		assert.Empty(t, cell(2, "uuid.UUID"), "zero uuid.UUID should be empty, not 00000000-...")
		assert.Empty(t, cell(2, "*uuid.UUID"), "nil *uuid.UUID should be empty")
		assert.Empty(t, cell(2, "time.Time"), "zero time.Time should be empty")
		assert.Empty(t, cell(2, "*time.Time"), "nil *time.Time should be empty")
		assert.Empty(t, cell(2, "*string"), "nil *string should be empty")
		assert.Empty(t, cell(2, "*int"), "nil *int should be empty")
		assert.Empty(t, cell(2, "IsZero() struct"), "IsZero()=true struct should be empty")
		assert.Empty(t, cell(2, "IsNil() struct"), "IsNil()=true struct should be empty")
	})

	t.Run("mixed row", func(t *testing.T) {
		assert.Equal(t, "Mixed", cell(3, "Field"))
		assert.Equal(t, uuidStr, cell(3, "uuid.UUID"))
		assert.Empty(t, cell(3, "*uuid.UUID"), "nil *uuid.UUID should be empty")
		assert.NotEmpty(t, cell(3, "time.Time"))
		assert.NotEqual(t, "{}", cell(3, "time.Time"))
		assert.Empty(t, cell(3, "*time.Time"))
		assert.Empty(t, cell(3, "*string"))
		assert.Empty(t, cell(3, "*int"))
		assert.Empty(t, cell(3, "IsZero() struct"), "IsZero()=true struct should be empty")
		assert.Equal(t, "nil:false", cell(3, "IsNil() struct"))
	})
}
