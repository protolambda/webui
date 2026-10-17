package web

import (
	"net/http"
	"strconv"
)

// HandlerFunc is a page or form handler that returns its failure instead of
// answering it: [Errors.Handle] turns it into an [http.Handler] that
// answers a returned error with an error page or an htmx error fragment.
//
// A handler that has started its response (written a status or a byte, or
// flushed) must not expect its error to be shown: the error is then logged
// only.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// Error is an error with the status and the message to answer it with.
//
// Message is shown to the user, on the error page or in the alert of an
// htmx request. It must be a fixed text chosen by the application: never
// put request data (form values, path segments, names) or secrets in it,
// as they would be reflected into the page. Empty means the
// [DefaultMessage] of the status.
//
// Err is the cause. It is logged, never shown.
type Error struct {
	// Status is the HTTP status of the answer, 400 to 599. Any other value
	// is answered with 500.
	Status int
	// Message is the public message; see the type documentation.
	Message string
	// Err is the cause, for the log only. It may be nil.
	Err error
}

// Error describes the error for logs: the status and the cause (or the
// public message when there is no cause).
func (e *Error) Error() string {
	if e == nil {
		return "web: <nil>"
	}
	s := "web: status " + strconv.Itoa(e.Status)
	switch {
	case e.Err != nil:
		return s + ": " + e.Err.Error()
	case e.Message != "":
		return s + ": " + e.Message
	}
	return s
}

// Unwrap returns the cause.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// StatusError returns an [Error] with the status, the public message and
// the cause. An empty message means the [DefaultMessage] of the status; a
// nil err means no cause.
func StatusError(status int, message string, err error) *Error {
	return &Error{Status: status, Message: message, Err: err}
}

// BadRequest returns a 400 Bad Request [Error], for a request that the
// handler refuses, such as an invalid form.
func BadRequest(message string, err error) *Error {
	return StatusError(http.StatusBadRequest, message, err)
}

// Forbidden returns a 403 Forbidden [Error], for an action that the user
// may not take.
func Forbidden(message string, err error) *Error {
	return StatusError(http.StatusForbidden, message, err)
}

// NotFound returns a 404 Not Found [Error], for a missing item.
func NotFound(message string, err error) *Error {
	return StatusError(http.StatusNotFound, message, err)
}

// Conflict returns a 409 Conflict [Error], for an action on an item that
// changed since the page showed it.
func Conflict(message string, err error) *Error {
	return StatusError(http.StatusConflict, message, err)
}

// Internal returns a 500 Internal Server Error [Error] with the default
// message: the cause is logged as an error.
func Internal(err error) *Error {
	return StatusError(http.StatusInternalServerError, "", err)
}

// DefaultMessage returns the public message of an error status without a
// message of its own: a short sentence that names no request data.
func DefaultMessage(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "The request is not valid."
	case http.StatusUnauthorized:
		return "Log in to continue."
	case http.StatusForbidden:
		return "This action is not allowed."
	case http.StatusNotFound:
		return "Nothing was found here."
	case http.StatusMethodNotAllowed:
		return "This action is not available here."
	case http.StatusConflict:
		return "The item changed meanwhile. Reload the page and try again."
	case http.StatusGone:
		return "This is no longer available."
	case http.StatusRequestEntityTooLarge:
		return "The request is too large."
	case http.StatusUnsupportedMediaType:
		return "The request has an unsupported format."
	case http.StatusUnprocessableEntity:
		return "The request could not be processed."
	case http.StatusTooManyRequests:
		return "Too many requests. Wait a moment and try again."
	case http.StatusInternalServerError:
		return "The request failed. The server log has details."
	case http.StatusServiceUnavailable:
		return "The service is not available. Try again later."
	case http.StatusGatewayTimeout:
		return "The request took too long. Try again later."
	}
	if text := http.StatusText(status); text != "" && status >= 400 && status <= 599 {
		return text + "."
	}
	return "The request failed."
}

// StatusTitle returns the title of an error page for status, e.g.
// "404 Not Found".
func StatusTitle(status int) string {
	if text := http.StatusText(status); text != "" {
		return strconv.Itoa(status) + " " + text
	}
	return "Error " + strconv.Itoa(status)
}
