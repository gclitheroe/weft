/*
weft helps with web applications.
 */
package weft

import (
	"net/http"
	"bytes"
	"strings"
)

// Return pointers to these as required.
var (
	StatusOK         = Result{Ok: true, Code: http.StatusOK, Msg: ""}
	MethodNotAllowed = Result{Ok: false, Code: http.StatusMethodNotAllowed, Msg: "method not allowed"}
	NotFound         = Result{Ok: false, Code: http.StatusNotFound, Msg: "not found"}
	NotAcceptable    = Result{Ok: false, Code: http.StatusNotAcceptable, Msg: "specify accept"}
)

type Result struct {
	Ok   bool   // set true to indicate success
	Code int    // http status code for writing back to the client e.g., http.StatusOK for success.
	Msg  string // any error message for logging or to send to the client.
}

type RequestHandler func(r *http.Request, h http.Header, b *bytes.Buffer) *Result

func InternalServerError(err error) *Result {
	return &Result{Ok: false, Code: http.StatusInternalServerError, Msg: err.Error()}
}

func ServiceUnavailableError(err error) *Result {
	return &Result{Ok: false, Code: http.StatusServiceUnavailable, Msg: err.Error()}
}

func BadRequest(message string) *Result {
	return &Result{Ok: false, Code: http.StatusBadRequest, Msg: message}
}

/*
CheckQuery inspects r and makes sure all required query parameters
are present and that no more than the required and optional parameters
are present.
*/
func CheckQuery(r *http.Request, required, optional []string) *Result {
	if strings.Contains(r.URL.Path, ";") {
		return BadRequest("cache buster")
	}

	v := r.URL.Query()

	if len(required) == 0 && len(optional) == 0 {
		if len(v) == 0 {
			return &StatusOK
		} else {
			return BadRequest("found unexpected query parameters")
		}
	}

	var missing []string

	for _, k := range required {
		if v.Get(k) == "" {
			missing = append(missing, k)
		} else {
			v.Del(k)
		}
	}

	switch len(missing) {
	case 0:
	case 1:
		return BadRequest("missing required query parameter: " + missing[0])
	default:
		return BadRequest("missing required query parameters: " + strings.Join(missing, ", "))
	}

	for _, k := range optional {
		v.Del(k)
	}

	if len(v) > 0 {
		return BadRequest("found additional query parameters")
	}

	return &StatusOK
}
