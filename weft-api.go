package weft

import (
	"fmt"
	"bytes"
	"strings"
)

type API struct {
	Endpoints []Endpoint
}

type Endpoint struct {
	URI    string
	GET    []Request
	PUT    *Request
	DELETE *Request
}

type Request struct {
	Func       RequestHandler
	Accept     string
	Default    bool // true to also add this as the default request when switching on Accept.
	Parameters Parameters
}

type Parameter struct {
	ID       string
	Required bool
}

func funcName(f string) string {
	if strings.HasSuffix(f, "/") {
		f = f + "s"
	}

	return strings.Replace(f, "/", "", -1) + "Handler"
}

// TODO add sort
type Parameters []Parameter

// check writes a checkQuery func to b for the parameters.
func (p Parameters) check(b *bytes.Buffer) {
	var req, opt []string

	for _, v := range p {
		if v.Required {
			req = append(req, v.ID)
		} else {
			opt = append(opt, v.ID)
		}
	}

	var r, o string

	if len(req) > 0 {
		r = `"` + strings.Join(req, `", "`) + `"`

	}

	if len(opt) > 0 {
		o = `"` + strings.Join(opt, `", "`) + `"`

	}

	b.WriteString(fmt.Sprintf("if res := weft.CheckQuery(r, []string{%s}, []string{%s}); !res.Ok {\n", r, o))
	b.WriteString("return res\n")
	b.WriteString("}\n")
}

func (a API) Handlers() (*bytes.Buffer, error) {
	var b bytes.Buffer

	for _, e := range a.Endpoints {
		// TODO check for duplicates
		if e.URI == "" {
			return &b, fmt.Errorf("found empty URI")
		}

		if len(e.GET) == 0 && e.DELETE == nil && e.PUT == nil {
			return &b, fmt.Errorf("found no requests (GET, PUT, DELETE) for %s", e.URI)
		}
	}

		b.WriteString(`package main` + "\n")
	b.WriteString("\n")
	b.WriteString("// This file is auto generated - do not edit.\n")
	b.WriteString("\n")
	b.WriteString(`import (` + "\n")
	b.WriteString(`"bytes"` + "\n")
	b.WriteString(`"github.com/GeoNet/weft"` + "\n")
	b.WriteString(`"net/http"` + "\n")
	b.WriteString(`)` + "\n")

	// the init() func - add routes the mux
	// assumes there is a var mux in the source elsewhere
	// we can't add it to the file built from this buffer or it
	// causes a circular dependency.

	b.WriteString("\n")
	b.WriteString("func init() {\n")

	for _, e := range a.Endpoints {
		b.WriteString(fmt.Sprintf("mux.HandleFunc(\"%s\", weft.MakeHandlerAPI(%s))\n", e.URI, funcName(e.URI)))
	}

	b.WriteString("}\n")
	b.WriteString("\n")

	// create the handler func for each endpoint with a method switch and an accept
	// switch for GET.

	for _, e := range a.Endpoints {
		b.WriteString(fmt.Sprintf("func %s(r *http.Request, h http.Header, b *bytes.Buffer) *weft.Result {\n", funcName(e.URI)))
		b.WriteString("switch r.Method {\n")

		if e.GET != nil && len(e.GET) >= 0 {
			b.WriteString(`case "GET":` + "\n")
			b.WriteString(`switch r.Header.Get("Accept") {` + "\n")

			var d Request
			var hasDefault bool

			for _, r := range e.GET {
				if r.Default {
					if hasDefault {
						return &b, fmt.Errorf("found multiple defaults for %s GET", e.URI)
					}
					hasDefault = true
					d = r
				}

				b.WriteString(fmt.Sprintf("case \"%s\":\n", r.Accept))
				r.Parameters.check(&b)
				b.WriteString(fmt.Sprintf("h.Set(\"Content-Type\", \"%s\")\n", r.Accept))
				b.WriteString(fmt.Sprintf("return %s(r, h, b)\n", name(r.Func)))
			}

			b.WriteString("default:\n")
			if hasDefault {
				// TODO set Content-Type and above as well.
				b.WriteString(fmt.Sprintf("return %s(r, h, b)\n", name(d.Func)))
			} else {
				b.WriteString("return &weft.NotAcceptable\n")
			}

			b.WriteString("}\n")
		}

		if e.PUT != nil {
			b.WriteString(`case "PUT":` + "\n")
			e.PUT.Parameters.check(&b)
			b.WriteString(fmt.Sprintf("return %s(r, h, b)\n", name(e.PUT.Func)))
		}

		if e.DELETE != nil {
			b.WriteString(`case "DELETE":` + "\n")
			e.DELETE.Parameters.check(&b)
			b.WriteString(fmt.Sprintf("return %s(r, h, b)\n", name(e.DELETE.Func)))
		}

		b.WriteString("default:\n")
		b.WriteString("return &weft.MethodNotAllowed\n")
		b.WriteString("}\n")
		b.WriteString("}\n")
		b.WriteString("\n")
	}

	return &b, nil
}