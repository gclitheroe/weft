package weft

import (
	"bytes"
	"compress/gzip"
	"log"
	"net/http"
	"strings"
)

// For setting Cache-Control and Surrogate-Control headers.
const (
	maxAge10    = "max-age=10"
	maxAge86400 = "max-age=86400"
)

// These constants are for error and other pages.
const (
	errContent  = "text/plain; charset=utf-8"
	htmlContent = "text/html; charset=utf-8"
)

var compressibleMimes = map[string]bool{
	// Compressible types from https://www.fastly.com/blog/new-gzip-settings-and-deciding-what-compress
	"text/html":                     true,
	"application/x-javascript":      true,
	"text/css":                      true,
	"application/javascript":        true,
	"text/javascript":               true,
	"text/plain":                    true,
	"text/xml":                      true,
	"application/json":              true,
	"application/vnd.ms-fontobject": true,
	"application/x-font-opentype":   true,
	"application/x-font-truetype":   true,
	"application/x-font-ttf":        true,
	"application/xml":               true,
	"font/eot":                      true,
	"font/opentype":                 true,
	"font/otf":                      true,
	"image/svg+xml":                 true,
	"image/vnd.microsoft.icon":      true,
	// other types
	"application/vnd.geo+json": true,
	"application/cap+xml":      true,
	"text/csv":                 true,
}

/*
MakeHandler executes f and writes the response to the client.

May not be suitable for middleware chaining.
*/
func MakeHandler(f RequestHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b bytes.Buffer

		w.Header().Set("Vary", "Accept")
		w.Header().Set("Surrogate-Control", maxAge10)

		// TODO add mtr monitoring
		res := f(r, w.Header(), &b)

		Write(w, r, res, &b)
	}
}

/*
Write writes the response to w.  The response is gzipped if appropriate for the client
and the content.  Appropriate response headers are set.  Surrogate-Control headers are
also set for intermediate caches.  Changes made to Surrogate-Control made before
calling Write will be respected for res.Code == 200 and overwritten for other Codes.

If b is non nil and non zero length then it's contents are always written to w.

If b is nil then only headers are written to w.

In the case of res.Code being for an error and b non nil then header "Weft-Error" is
checked.  When it is 'page' an html page is written to the client.
 When it is 'msg' (or empty) then res.Msg is written to the client.

Weft-Error is removed from the header before writing to the client.
*/
func Write(w http.ResponseWriter, r *http.Request, res *Result, b *bytes.Buffer) {
	if res.Code == 0 {
		res.Code = http.StatusOK
		log.Printf("ERROR: weft - received Result.Code == 0, serving 200.")
	}

	if res.Code != 200 {

		switch w.Header().Get("Weft-Error") {
		case "page":
			w.Header().Set("Content-Type", htmlContent)
			if b != nil && b.Len() == 0 {
				if e, ok := errorPages[res.Code]; ok {
					b.Write(e)
				} else {
					e, _ := errorPages[http.StatusInternalServerError]
					b.Write(e)
				}
			}
		case "msg", "":
			w.Header().Set("Content-Type", errContent)

			if b != nil && b.Len() == 0 {
				b.WriteString(res.Msg)
			}
		}

		switch res.Code {
		case http.StatusNotFound:
			w.Header().Set("Surrogate-Control", maxAge10)
		case http.StatusServiceUnavailable:
			w.Header().Set("Surrogate-Control", maxAge10)
		case http.StatusInternalServerError:
			w.Header().Set("Surrogate-Control", maxAge10)
		case http.StatusBadRequest:
			w.Header().Set("Surrogate-Control", maxAge86400)
		case http.StatusMethodNotAllowed:
			w.Header().Set("Surrogate-Control", maxAge86400)
		default:
			w.Header().Set("Surrogate-Control", maxAge10)
		}
	}

	w.Header().Del("Weft-Error")

	// write the response.  With gzipping if possible.
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", http.DetectContentType(b.Bytes()))
	}

	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") && b != nil && b.Len() > 20 {

		contentType := w.Header().Get("Content-Type")

		i := strings.Index(contentType, ";")
		if i > 0 {
			contentType = contentType[0:i]
		}

		contentType = strings.TrimSpace(contentType)

		if compressibleMimes[contentType] {
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			w.WriteHeader(res.Code)
			b.WriteTo(gz)
		}
	} else {
		w.WriteHeader(res.Code)
		if b != nil {
			b.WriteTo(w)
		}
	}
}
