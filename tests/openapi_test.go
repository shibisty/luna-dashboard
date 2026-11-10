package tests

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	apptest "fullstack-testing"

	"luna-dashboard/docs"
)

// specOperations reads "METHOD /path" of every operation in the OpenAPI
// document (paths at two spaces, methods at four).
func specOperations(t *testing.T, spec string) map[string]bool {
	t.Helper()
	pathRe := regexp.MustCompile(`^  (/\S*):\s*$`)
	methodRe := regexp.MustCompile(`^    (get|put|post|patch|delete):\s*$`)
	ops := map[string]bool{}
	inPaths, current := false, ""
	for _, line := range strings.Split(spec, "\n") {
		switch {
		case line == "paths:":
			inPaths = true
		case inPaths && line != "" && !strings.HasPrefix(line, " "):
			inPaths = false
		case inPaths:
			if m := pathRe.FindStringSubmatch(line); m != nil {
				current = m[1]
			} else if m := methodRe.FindStringSubmatch(line); m != nil && current != "" {
				ops[strings.ToUpper(m[1])+" "+current] = true
			}
		}
	}
	if len(ops) == 0 {
		t.Fatal("no operations found in the OpenAPI document")
	}
	return ops
}

// TestOpenAPIMatchesRoutes keeps docs/openapi.yaml and the machine routes
// (/healthz, /internal/v1/...) in step; the HTML pages are not an API.
func TestOpenAPIMatchesRoutes(t *testing.T) {
	app := newApp(t)
	documented := specOperations(t, string(docs.OpenAPI))
	routes := map[string]bool{}
	for _, r := range app.Router.Routes() {
		api := r.Path == "/healthz" || strings.HasPrefix(r.Path, "/internal/v")
		if api && r.Method != "HEAD" {
			routes[r.Method+" "+r.Path] = true
		}
	}
	var missing, extra []string
	for op := range routes {
		if !documented[op] {
			missing = append(missing, op)
		}
	}
	for op := range documented {
		if !routes[op] {
			extra = append(extra, op)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("docs/openapi.yaml is out of step with the routes:\n  routes not documented: %v\n  documented, no route: %v", missing, extra)
	}
}

func TestDocsServed(t *testing.T) {
	c := apptest.New(t, newApp(t))
	c.Get("/internal/openapi.yaml").AssertOK().AssertSee("openapi: 3.1.0", "/internal/v1/keys/introspect")
	c.Get("/internal/docs").AssertNotFound() // API_DOCS is off
}
