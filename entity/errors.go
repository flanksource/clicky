package entity

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/flanksource/commons/logger"
)

// InternalErrorMessage is the body message every unclassified failure is
// reported with, so an internal detail can never reach a client by accident.
const InternalErrorMessage = "internal server error"

// StatusError is the body every failed HTTP operation returns.
//
// The 409 that asks for a connection has always been structured, and a client
// that can branch on one code but has to string-match the rest is a client that
// breaks when a message is reworded. Code is the stable part; Message is for a
// person to read. It replaces http.Error, whose flat text body forces exactly
// the string-matching this exists to avoid.
type StatusError struct {
	// Status is the HTTP status this error is written with. It never appears in
	// the body: the status line already carries it, and repeating it invites the
	// two to disagree.
	Status int `json:"-"`

	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewStatusError builds an error a handler can return and the transport can
// write verbatim.
func NewStatusError(status int, code, message string) *StatusError {
	return &StatusError{Status: status, Code: code, Message: message}
}

// NewStatusErrorf is NewStatusError with a formatted message.
func NewStatusErrorf(status int, code, format string, args ...any) *StatusError {
	return &StatusError{Status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// StatusCode reports the status this error should be written with, defaulting
// to 500 so a zero Status can never produce an invalid status line.
func (e *StatusError) StatusCode() int {
	if e.Status == 0 {
		return http.StatusInternalServerError
	}
	return e.Status
}

// Write renders the error as its JSON body at its status.
func (e *StatusError) Write(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.StatusCode())
	_ = json.NewEncoder(w).Encode(e)
}

// WriteError writes any error in the same shape, so a caller never has to
// choose between two error formats depending on where the failure came from.
//
// An error that did not state a status is a bug rather than a refusal, so it
// becomes a 500 with an opaque code and a fixed message. Its own text is not
// forwarded: an unclassified error is whatever the failing dependency said, and
// that routinely names hosts, DSNs, file paths, or credentials. The detail stays
// server-side in the log; a handler that wants the client to see a reason must
// classify the failure with a StatusError.
func WriteError(w http.ResponseWriter, err error) {
	var status *StatusError
	if errors.As(err, &status) {
		status.Write(w)
		return
	}
	logger.Errorf("unclassified handler error: %v", err)
	NewStatusError(http.StatusInternalServerError, "internal_error", InternalErrorMessage).Write(w)
}
