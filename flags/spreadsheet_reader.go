package flags

import (
	"errors"
	"io"
	"sync"
)

// SpreadsheetReader returns the first worksheet's name and its rows. It backs
// @data.xlsx:Column flag values; opt in with excel.Use() from
// github.com/flanksource/clicky/formatters/excel.
type SpreadsheetReader func(r io.Reader) (sheet string, rows [][]string, err error)

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

func readSpreadsheet(r io.Reader) (string, [][]string, error) {
	spreadsheetReaderMu.RLock()
	reader := spreadsheetReader
	spreadsheetReaderMu.RUnlock()
	if reader == nil {
		return "", nil, ErrNoSpreadsheetReader
	}
	return reader(r)
}
