package broker

import (
	"strings"
	"testing"

	"code.cloudfoundry.org/cli/v8/resources"
)

func TestBuildBindingCredentials(t *testing.T) {
	routes := []resources.Route{{URL: "my-app.apps.example.com"}}

	creds, err := buildBindingCredentials(routes, "https://uaa.example.com", "client-id", "s3cret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]string{
		"uri":              "https://my-app.apps.example.com",
		"access_token_uri": "https://uaa.example.com/oauth/token",
		"client_id":        "client-id",
		"client_secret":    "s3cret",
	}
	for k, v := range want {
		if creds[k] != v {
			t.Errorf("creds[%q] = %q, want %q", k, creds[k], v)
		}
	}
}

// A backing app that failed to stage has no route. Bind must return a clean
// error rather than panicking on routes[0] (see plan decision D4).
func TestBuildBindingCredentialsNoRoutes(t *testing.T) {
	for _, routes := range [][]resources.Route{nil, {}} {
		creds, err := buildBindingCredentials(routes, "https://uaa.example.com", "client-id", "s3cret")
		if err == nil {
			t.Fatalf("expected an error for %d routes, got credentials %v", len(routes), creds)
		}
		if !strings.Contains(err.Error(), "no route") {
			t.Errorf("error %q should explain the app has no route", err)
		}
	}
}
