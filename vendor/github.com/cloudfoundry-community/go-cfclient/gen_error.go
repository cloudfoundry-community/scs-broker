//go:build tools
// +build tools

package main

import (
	"bytes"
	"go/format"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v2"
)

type (
	CFCode   int
	HTTPCode int
)

type Definition struct {
	CFCode   `yaml:"-"`
	Name     string `yaml:"name"`
	HTTPCode `yaml:"http_code"`
	Message  string `yaml:"message"`
}

func main() {
	log.SetFlags(log.Lshortfile)
	const url = "https://raw.githubusercontent.com/cloudfoundry/cloud_controller_ng/master/errors/v2.yml"
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var m map[CFCode]Definition

	if err := yaml.Unmarshal(body, &m); err != nil {
		log.Fatal(err)
	}

	var definitions []Definition

	for c, d := range m {
		d.CFCode = c
		definitions = append(definitions, d)
	}

	sort.Slice(definitions, func(i, j int) bool {
		return definitions[i].CFCode < definitions[j].CFCode
	})

	buf := &bytes.Buffer{}

	if err := packageTemplate.Execute(buf, struct {
		Timestamp   time.Time
		Definitions []Definition
	}{
		Timestamp:   time.Now(),
		Definitions: definitions,
	}); err != nil {
		log.Fatal(err)
	}

	dst, err := format.Source(buf.Bytes())
	if err != nil {
		log.Printf("%s", buf.Bytes())
		log.Fatal(err)
	}

	if err := ioutil.WriteFile("cf_error.go", dst, 0600); err != nil {
		log.Fatal(err)
	}
}

// destutter ensures that s does not end in "Error".
func destutter(s string) string {
	return strings.TrimSuffix(s, "Error")
}

// cleanMessage removes any characters which will cause go generate to fail
func cleanMessage(s string) string {
	return strings.Replace(s, "\n", "", -1)
}

var packageTemplate = template.Must(template.New("").Funcs(template.FuncMap{
	"destutter":    destutter,
	"cleanMessage": cleanMessage,
}).Parse(`
package cfclient

// Code generated by go generate. DO NOT EDIT.
// This file was generated by robots at
// {{ .Timestamp }}

import (
	stderrors "errors"

	pkgerrors "github.com/pkg/errors"
)

{{- range .Definitions }}
{{$isMethod := printf "Is%sError" (.Name | destutter) }}
{{$newMethod := printf "New%sError" (.Name | destutter) }}
// {{ $newMethod }} returns a new CloudFoundryError
// that {{ $isMethod }} will return true for
func {{ $newMethod }}() CloudFoundryError {
	return CloudFoundryError{
		Code: {{ .CFCode }},
		ErrorCode: "CF-{{ .Name }}",
		Description: "{{ .Message | cleanMessage }}",
	}
}

// {{ $isMethod }} returns a boolean indicating whether
// the error is known to report the Cloud Foundry error:
// - Cloud Foundry code: {{ .CFCode }}
// - HTTP code: {{ .HTTPCode }}
// - message: {{ printf "%q" .Message }}
func Is{{ .Name | destutter }}Error(err error) bool {
	cferr, ok := cloudFoundryError(err)
	if !ok {
		return false
	}
	return cferr.Code == {{ .CFCode }}
}
{{- end }}

func cloudFoundryError(err error) (cferr CloudFoundryError, ok bool) {
	type causer interface {
		Cause() error
	}
	if _, isCauser := err.(causer); isCauser {
		cause := pkgerrors.Cause(err)
		cferr, ok = cause.(CloudFoundryError)
	} else {
		ok = stderrors.As(err, &cferr)
	}
	return cferr, ok
}
`))
