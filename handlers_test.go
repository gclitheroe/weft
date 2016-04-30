package weft

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"testing"
)

/*
TestWriteSurrogate checks the Surrogate-Control headers are correct.  This
header is used to control the behaviour of front end caches.
*/
func TestWriteSurrogate(t *testing.T) {
	var w *httptest.ResponseRecorder

	r, err := http.NewRequest("GET", "http://test.com", nil)
	if err != nil {
		t.Fatal(err)
	}

	res := Result{}
	var b bytes.Buffer

	// unset res.Code defaults to 200 response behaviour
	w = httptest.NewRecorder()
	Write(w, r, &res, &b)
	checkResponse(t, w, http.StatusOK, "max-age=10", "", "")

	res.Code = http.StatusOK
	w = httptest.NewRecorder()
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "", "")

	res.Code = http.StatusNotFound
	w = httptest.NewRecorder()
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "", "")

	res.Code = http.StatusServiceUnavailable
	w = httptest.NewRecorder()
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "", "")

	res.Code = http.StatusInternalServerError
	w = httptest.NewRecorder()
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "", "")

	res.Code = http.StatusMethodNotAllowed
	w = httptest.NewRecorder()
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=86400", "", "")

	res.Code = http.StatusBadRequest
	w = httptest.NewRecorder()
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=86400", "", "")

	// unknown code gets default cache time
	res.Code = 999
	w = httptest.NewRecorder()
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "", "")
}

/*
TestWriteGzip checks Accept-Encoding header and gzipping the response
is handled correctly
*/
func TestWriteGzip(t *testing.T) {
	var w *httptest.ResponseRecorder

	r, err := http.NewRequest("GET", "http://test.com", nil)
	if err != nil {
		t.Fatal(err)
	}

	res := Result{}
	var b bytes.Buffer

	// gzip request with nil buffer does not get compressed.
	res.Code = http.StatusOK
	w = httptest.NewRecorder()
	r.Header.Set("Accept-Encoding", "deflate, gzip")
	Write(w, r, &res, nil)
	checkResponse(t, w, res.Code, "max-age=10", "", "")

	// gzip request with zero length buffer does not get compressed.
	res.Code = http.StatusOK
	w = httptest.NewRecorder()
	r.Header.Set("Accept-Encoding", "deflate, gzip")
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "", "")

	// gzip request with length buffer < 20 does not get compressed.
	b.Reset()
	b.WriteString("bogan impsum")
	e := b.String()

	res.Code = http.StatusOK
	w = httptest.NewRecorder()
	r.Header.Set("Accept-Encoding", "deflate, gzip")
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "", e)

	// gzip request with length buffer > 20 gets compressed.
	b.Reset()
	b.WriteString("bogan impsum bogan impsum")
	b.WriteString("bogan impsum bogan impsum")
	b.WriteString("bogan impsum bogan impsum")

	len := b.Len()
	e = b.String()

	res.Code = http.StatusOK
	w = httptest.NewRecorder()
	r.Header.Set("Accept-Encoding", "deflate, gzip")
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "gzip", e)

	if w.Body.Len() >= len {
		t.Error("gzip didn't happen?")
	}

	// non gzip request with length buffer > 20 does not get gzipped.
	b.Reset()
	b.WriteString("bogan impsum bogan impsum")
	b.WriteString("bogan impsum bogan impsum")
	b.WriteString("bogan impsum bogan impsum")

	e = b.String()
	len = b.Len()

	res.Code = http.StatusOK
	w = httptest.NewRecorder()
	r.Header.Del("Accept-Encoding")
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "", e)

	if w.Body.Len() != len {
		t.Error("gzip shouldn't happen?")
	}

	// gzip request with non compressible content type
	// does not get compressed.
	b.Reset()
	b.WriteString("bogan impsum bogan impsum")
	b.WriteString("bogan impsum bogan impsum")
	b.WriteString("bogan impsum bogan impsum")

	len = b.Len()
	e = b.String()

	res.Code = http.StatusOK
	w = httptest.NewRecorder()
	r.Header.Set("Accept-Encoding", "deflate, gzip")
	w.Header().Set("Content-Type", "image/png")
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "", e)
	if w.Body.Len() != len {
		t.Error("gzip shouldn't happen?")
	}
}

/*
TestErrorResponses checks behaviour with the Weft-Error marker.
*/
func TestErrorResponses(t *testing.T) {
	var w *httptest.ResponseRecorder

	r, err := http.NewRequest("GET", "http://test.com", nil)
	if err != nil {
		t.Fatal(err)
	}

	res := Result{}
	var b bytes.Buffer

	// default behaviour should write res.Msg for an error
	res.Code = http.StatusNotFound
	res.Msg = "error message"
	w = httptest.NewRecorder()
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "", res.Msg)

	// should get res.Msg even with non empty buffer
	b.WriteString("non empty")
	w = httptest.NewRecorder()
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "", res.Msg)

	r.Header.Set("Weft-Error", "msg")
	w = httptest.NewRecorder()
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "", res.Msg)

	// error pages
	r.Header.Set("Weft-Error", "page")

	w = httptest.NewRecorder()
	res.Code = 0
	b.Reset()
	b.WriteString("non empty")
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "", "non empty")


	w = httptest.NewRecorder()
	res.Code = http.StatusNotFound
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "", err404)

	w = httptest.NewRecorder()
	res.Code = http.StatusInternalServerError
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "", err503)

	w = httptest.NewRecorder()
	res.Code = http.StatusServiceUnavailable
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=10", "", err503)

	w = httptest.NewRecorder()
	res.Code = http.StatusBadRequest
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=86400", "", err400)

	w = httptest.NewRecorder()
	res.Code = http.StatusMethodNotAllowed
	Write(w, r, &res, &b)
	checkResponse(t, w, res.Code, "max-age=86400", "", err405)

	w = httptest.NewRecorder()
	res.Code = 999
	Write(w, r, &res, &b)
	checkResponse(t, w, 999, "max-age=10", "", err503)
}

func checkResponse(t *testing.T, w *httptest.ResponseRecorder, code int, surrogate, encoding, body string) {
	l := loc()

	if w.Code != code {
		t.Errorf("%s wrong status code expected %d got %d", l, code, w.Code)
	}

	if w.Header().Get("Surrogate-Control") != surrogate {
		t.Errorf("%s wrong Surrogate-Control, expected %s got %s", l, surrogate, w.Header().Get("Surrogate-Control"))
	}

	if w.Header().Get("Content-Encoding") != encoding {
		t.Errorf("%s wrong Content-Encoding expected %s got %s", l, encoding, w.Header().Get("Content-Encoding"))
	}

	if w.Header().Get("Weft-Error") != "" {
		t.Errorf("% unexpected Weft-Error header: %s", l, w.Header().Get("Weft-Error"))
	}

	switch w.Header().Get("Content-Encoding") {
	case "gzip":
		gz, err := gzip.NewReader(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		defer gz.Close()

		var b bytes.Buffer
		b.ReadFrom(gz)

		if b.String() != body {
			t.Errorf("%s got wrong body", l)
		}
	default:
		if w.Body.String() != body {
			t.Errorf("%s got wrong body", l)
		}
	}
}

// loc returns a string representing the line of code 2 functions calls back.
func loc() (loc string) {
	_, _, l, _ := runtime.Caller(2)
	return "L" + strconv.Itoa(l)
}
