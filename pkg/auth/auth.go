package auth

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// TokenSource provides Google ID tokens and user identity for API authentication.
// Tokens are cached and refreshed automatically when they expire.
type TokenSource struct {
	mu        sync.Mutex
	token     string
	userEmail string
	expiry    time.Time
}

// defaultTokenLifetime is the assumed lifetime of a gcloud identity token (1 hour).
const defaultTokenLifetime = 55 * time.Minute

// NewTokenSource creates a TokenSource that acquires Google ID tokens
// via gcloud without a custom audience (suitable for API Gateway auth).
func NewTokenSource() *TokenSource {
	return &TokenSource{}
}

// Token returns a valid Google ID token and the authenticated user's email,
// refreshing both if necessary.
func (ts *TokenSource) Token() (token, userEmail string, err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.token != "" && time.Now().Before(ts.expiry) {
		return ts.token, ts.userEmail, nil
	}

	ts.token, err = fetchIdentityToken()
	if err != nil {
		return "", "", err
	}

	ts.userEmail, _ = fetchAccountEmail()
	ts.expiry = time.Now().Add(defaultTokenLifetime)
	return ts.token, ts.userEmail, nil
}

// fetchIdentityToken calls gcloud to obtain a Google ID token without a
// custom audience, matching the API Gateway's expected token format.
func fetchIdentityToken() (string, error) {
	cmd := exec.Command("gcloud", "auth", "print-identity-token")
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		if stderr != "" {
			return "", fmt.Errorf("failed to get identity token: %s", stderr)
		}
		return "", fmt.Errorf("failed to get identity token: %w\n\n"+
			"  Ensure gcloud is installed and authenticated:\n"+
			"    gcloud auth login", err)
	}

	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("gcloud returned an empty identity token")
	}
	return token, nil
}

// fetchAccountEmail calls gcloud to retrieve the active account's email.
func fetchAccountEmail() (string, error) {
	cmd := exec.Command("gcloud", "config", "get-value", "account")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get account email: %w", err)
	}
	email := strings.TrimSpace(string(out))
	if email == "" || email == "(unset)" {
		return "", fmt.Errorf("no active gcloud account")
	}
	return email, nil
}
