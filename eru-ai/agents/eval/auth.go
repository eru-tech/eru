package eval

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Login exchanges a username and password for the claims header.
//
// Behind the gateway, a caller's token is turned into `claims` for the services
// to read; called directly there is no gateway, so the harness has to produce
// the same header itself. It is the id_token's payload verbatim - the same
// derivation claims.util.ts performs for localhost - so a run authenticates
// exactly as a person would rather than carrying a hand-copied blob that goes
// stale without anyone noticing.
func Login(ctx context.Context, client *http.Client, authURL, project, username, password string) (string, error) {
	body, err := json.Marshal(map[string]string{"username": username, "password": hashPassword(password)})
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s/%s/eru/login", strings.TrimRight(authURL, "/"), project)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	var reply struct {
		IdToken string `json:"id_token"`
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&reply); err != nil {
		return "", fmt.Errorf("decoding the login reply: %w", err)
	}
	if reply.Error != "" {
		// Said plainly: this is the most likely thing to be wrong when a suite
		// that ran yesterday stops running today.
		return "", fmt.Errorf("login as %s was refused: %s", username, reply.Error)
	}
	if reply.IdToken == "" {
		return "", fmt.Errorf("login as %s returned no id_token", username)
	}
	return claimsFromIdToken(reply.IdToken)
}

// hashPassword is what the login screen sends: the password never leaves the
// client in the clear, so a caller that posts the raw one is simply refused with
// "invalid credentials" and no hint as to why. SHA-512, lowercase hex - the same
// derivation utils.service.ts performs before it calls login.
func hashPassword(password string) string {
	sum := sha512.Sum512([]byte(password))
	return hex.EncodeToString(sum[:])
}

// claimsFromIdToken is the JWT's payload, decoded. No signature check: this is
// not authenticating anyone, it is reproducing a header the gateway would have
// written from a token the auth service just issued.
func claimsFromIdToken(idToken string) (string, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("id_token is not a JWT")
	}
	payload := parts[1]
	if pad := len(payload) % 4; pad != 0 {
		payload += strings.Repeat("=", 4-pad)
	}
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("decoding the id_token payload: %w", err)
	}
	// Re-serialised so a malformed payload fails here, where the message is
	// clear, rather than as an empty result from a query three calls later.
	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return "", fmt.Errorf("the id_token payload is not JSON: %w", err)
	}
	out, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
