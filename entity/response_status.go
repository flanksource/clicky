package entity

// ResponseStatus is implemented by an operation result that succeeds with a
// status other than 200 OK, such as 202 Accepted for work it started in the
// background. Only 2xx statuses are valid: a failure is an error, classified
// with NewStatusError, never a result.
type ResponseStatus interface {
	ResponseStatus() int
}
