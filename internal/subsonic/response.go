package subsonic

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"

	"rainy/internal/auth"
	"rainy/internal/buildinfo"
	"rainy/internal/scanner"
	"rainy/internal/store"
)

// Subsonic error codes.
const (
	codeGeneric          = 0
	codeMissingParam     = 10
	codeWrongCredentials = 40
	// 41 (token auth not supported) and 42 (auth mechanism not supported) are never
	// returned: every mechanism (password, token, API key) is supported.
	codeConflictingAuth = 43
	codeInvalidAPIKey   = 44
	codeNotAuthorized   = 50
	codeNotFound        = 70
)

// apiError is an error carrying a Subsonic error code.
type apiError struct {
	code int
	msg  string
}

func (e *apiError) Error() string { return fmt.Sprintf("subsonic error %d: %s", e.code, e.msg) }

func newError(code int, format string, args ...any) *apiError {
	return &apiError{code: code, msg: fmt.Sprintf(format, args...)}
}

func errMissing(param string) *apiError {
	return newError(codeMissingParam, "Required parameter is missing: %s", param)
}

func errNotFound(what string) *apiError {
	return newError(codeNotFound, "%s not found", what)
}

func errForbidden() *apiError {
	return newError(codeNotAuthorized, "User is not authorized for the given operation")
}

// toAPIError maps any error to a Subsonic error (logging unexpected ones).
func toAPIError(r *http.Request, err error) *apiError {
	var ae *apiError
	switch {
	case errors.As(err, &ae):
		return ae
	case errors.Is(err, store.ErrNotFound):
		return newError(codeNotFound, "The requested data was not found")
	case errors.Is(err, store.ErrInvalid), errors.Is(err, auth.ErrValidation), errors.Is(err, store.ErrConflict):
		return newError(codeGeneric, "%s", err.Error())
	case errors.Is(err, scanner.ErrScanInProgress):
		return newError(codeGeneric, "%s", err.Error())
	case errors.Is(err, errors.ErrUnsupported):
		return newError(codeGeneric, "Not implemented")
	case errors.Is(err, context.Canceled):
		return newError(codeGeneric, "Request cancelled")
	}
	slog.Error("subsonic: request failed", "path", r.URL.Path, "err", err)
	return newError(codeGeneric, "Internal server error")
}

// newResponse returns an "ok" envelope.
func newResponse() *Response {
	return &Response{
		Xmlns:         xmlNamespace,
		Status:        "ok",
		Version:       apiVersion,
		Type:          serverType,
		ServerVersion: buildinfo.Version,
		OpenSubsonic:  true,
	}
}

// failedResponse returns a "failed" envelope carrying e.
func failedResponse(e *apiError) *Response {
	resp := newResponse()
	resp.Status = "failed"
	resp.Error = &Error{Code: e.code, Message: e.msg}
	return resp
}

var callbackRe = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$.]{0,127}$`)

// writeResponse serialises resp in the requested format: "json", "jsonp" (with a valid
// callback) or XML (default). Subsonic always answers HTTP 200, errors included.
func writeResponse(w http.ResponseWriter, format, callback string, status int, resp *Response) {
	var body bytes.Buffer
	var contentType string
	switch format {
	case "json", "jsonp":
		b, err := json.Marshal(map[string]*Response{"subsonic-response": resp})
		if err != nil {
			slog.Error("subsonic: encoding JSON", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if format == "jsonp" && callbackRe.MatchString(callback) {
			contentType = "application/javascript; charset=utf-8"
			body.WriteString("/**/" + callback + "(")
			body.Write(b)
			body.WriteString(");")
		} else {
			contentType = "application/json; charset=utf-8"
			body.Write(b)
		}
	default:
		b, err := xml.Marshal(resp)
		if err != nil {
			slog.Error("subsonic: encoding XML", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		contentType = "text/xml; charset=utf-8"
		body.WriteString(xml.Header)
		body.Write(b)
	}
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write(body.Bytes())
}
