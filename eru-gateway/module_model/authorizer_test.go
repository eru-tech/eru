package module_model

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func TestMain(m *testing.M) {
	logs.LogInit("test", "gateway-authorizer-test")
	os.Exit(m.Run())
}

func TestReadExchangedToken(t *testing.T) {
	authorizer := Authorizer{TokenKey: "X-Token"}

	tests := []struct {
		name    string
		res     interface{}
		headers http.Header
		want    string
		wantErr bool
	}{
		{"token in the body", map[string]interface{}{"X-Token": "jwt.value"}, http.Header{}, "jwt.value", false},
		{"token in a response header", map[string]interface{}{"status": "ok"}, http.Header{"X-Token": []string{"jwt.value"}}, "jwt.value", false},
		{"body wins over header", map[string]interface{}{"X-Token": "from.body"}, http.Header{"X-Token": []string{"from.header"}}, "from.body", false},
		// these used to fall through silently and leave the access token's claims in place
		{"key missing from the body", map[string]interface{}{"token": "jwt.value"}, http.Header{}, "", true},
		{"key present but empty", map[string]interface{}{"X-Token": ""}, http.Header{}, "", true},
		{"key present but not a string", map[string]interface{}{"X-Token": 42}, http.Header{}, "", true},
		{"response is not an object", []interface{}{"jwt.value"}, http.Header{}, "", true},
	}
	for _, tt := range tests {
		got, err := authorizer.readExchangedToken(context.Background(), tt.res, tt.headers)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", tt.name, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("%s: token = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// the logged detail has to name the keys that did come back, and never the values - they are
// tokens. The error handed back to the caller stays generic, which is what logs.Err does.
func TestMissingExchangedTokenErrNamesKeysNotValues(t *testing.T) {
	err := missingExchangedTokenErr("X-Token", map[string]interface{}{"access_token": "secret.jwt.value", "status": "ok"})
	if !strings.Contains(err.Error(), "access_token") || !strings.Contains(err.Error(), "status") {
		t.Errorf("detail should name the body keys, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "secret.jwt.value") {
		t.Errorf("detail leaked a token value: %q", err.Error())
	}

	err = missingExchangedTokenErr("X-Token", []interface{}{"secret.jwt.value"})
	if !strings.Contains(err.Error(), "[]interface {}") {
		t.Errorf("detail should name the response type, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "secret.jwt.value") {
		t.Errorf("detail leaked a token value: %q", err.Error())
	}
}

// whatever the detail says, the caller never sees the internals
func TestReadExchangedTokenDoesNotLeakToCaller(t *testing.T) {
	authorizer := Authorizer{TokenKey: "X-Token"}
	_, err := authorizer.readExchangedToken(context.Background(), map[string]interface{}{"access_token": "secret.jwt.value"}, http.Header{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "secret.jwt.value") {
		t.Errorf("error returned to the caller leaked a token value: %q", err.Error())
	}
}
