// Package apperr defines the application error type shared by all layers.
// The HTTP layer maps it to a JSON error body; everything else is a 500.
package apperr

import (
	"fmt"
	"net/http"
)

type Error struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func New(status int, code, msg string) *Error {
	return &Error{Status: status, Code: code, Message: msg}
}

func BadRequest(code, msg string) *Error { return New(http.StatusBadRequest, code, msg) }
func Unauthorized(msg string) *Error     { return New(http.StatusUnauthorized, "unauthorized", msg) }
func Forbidden(msg string) *Error        { return New(http.StatusForbidden, "forbidden", msg) }
func NotFound(msg string) *Error         { return New(http.StatusNotFound, "not_found", msg) }
func Conflict(code, msg string) *Error   { return New(http.StatusConflict, code, msg) }
func TooLarge(msg string) *Error         { return New(http.StatusRequestEntityTooLarge, "too_large", msg) }

// Validation builds a 422 error with per-field messages.
func Validation(fields map[string]string) *Error {
	return &Error{Status: http.StatusUnprocessableEntity, Code: "validation_failed", Message: "Ma'lumotlar noto'g'ri to'ldirilgan", Fields: fields}
}

// Field is a shortcut for a single-field validation error.
func Field(name, msg string) *Error { return Validation(map[string]string{name: msg}) }
