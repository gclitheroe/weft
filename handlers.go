package weft

import (
	"bytes"
	"compress/gzip"
	"log"
	"net/http"
	"strings"
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

var surrogateControl = map[int]string{
	http.StatusNotFound:            "max-age=10",
	http.StatusServiceUnavailable:  "max-age=10",
	http.StatusInternalServerError: "max-age=10",
	http.StatusBadRequest:          "max-age=86400",
	http.StatusMethodNotAllowed:    "max-age=86400",
}

/*
MakeHandler executes f and writes the response to the client.
*/
func MakeHandler(f RequestHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b bytes.Buffer

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

If b is nil then only headers are written to w.

In the case of res.Code being for an error and b non nil then header "Weft-Error" is
checked.  When it is 'page' an html page is written to the client.  When
it is 'msg' (or empty) then res.Msg is written to the client.

Weft-Error is removed from the header before writing to the client.
*/
func Write(w http.ResponseWriter, r *http.Request, res *Result, b *bytes.Buffer) {
	if res.Code == 0 {
		res.Code = http.StatusOK
		log.Printf("WARN: weft - received Result.Code == 0, serving 200.")
	}

	if w.Header().Get("Surrogate-Control") == "" {
		w.Header().Set("Surrogate-Control", "max-age=10")
	}

	if res.Code != 200 {
		switch r.Header.Get("Weft-Error") {
		case "page":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if b != nil {
				b.Reset()
				if e, ok := errorPages[res.Code]; ok {
					b.Write(e)
				} else {
					b.Write(errorPages[http.StatusInternalServerError])
				}
			}
		case "msg", "":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")

			if b != nil {
				b.Reset()
				b.WriteString(res.Msg)
			}
		}

		if s, ok := surrogateControl[res.Code]; ok {
			w.Header().Set("Surrogate-Control", s)
		} else {
			w.Header().Set("Surrogate-Control", "max-age=10")
		}
	}

	w.Header().Del("Weft-Error")

	/*
	 write the response.  With gzipping if possible.
	*/

	w.Header().Add("Vary", "Accept-Encoding")

	if w.Header().Get("Content-Type") == "" && b != nil {
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

			return
		}
	}

	w.WriteHeader(res.Code)
	if b != nil {
		b.WriteTo(w)
	}
}
