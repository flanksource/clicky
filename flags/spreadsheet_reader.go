package flags

import (
	"errors"
	"io"
	"sync"
)

// SpreadsheetReadOptions selects a worksheet by name or one-based index.
// With neither set, the reader uses the first worksheet.
type SpreadsheetReadOptions struct {
	Sheet string
	Index int
}

// SpreadsheetReader returns the selected worksheet's name and its rows. It backs
// @data.xlsx:Sheet!Column flag values; opt in with excel.Use() from
// github.com/flanksource/clicky/formatters/excel.
type SpreadsheetReader func(r io.Reader, options SpreadsheetReadOptions) (sheet string, rows [][]string, err error)

var ErrNoSpreadsheetReader = errors.New("no spreadsheet reader configured: call excel.Use() from github.com/flanksource/clicky/formatters/excel")

var (
	spreadsheetReaderMu sync.RWMutex
	spreadsheetReader   SpreadsheetReader
)

func SetSpreadsheetReader(reader SpreadsheetReader) {
	spreadsheetReaderMu.Lock()
	defer spreadsheetReaderMu.Unlock()
	spreadsheetReader = reader
}

func readSpreadsheet(r io.Reader, options SpreadsheetReadOptions) (string, [][]string, error) {
	spreadsheetReaderMu.RLock()
	reader := spreadsheetReader
	spreadsheetReaderMu.RUnlock()
	if reader == nil {
		return "", nil, ErrNoSpreadsheetReader
	}
	return reader(r, options)
}
