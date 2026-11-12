package handlers

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"strings"
	"testing"
	"time"
)

// generateTestServiceAccountKey returns a PKCS#8 PEM key and the RSA key it
// contains, with newlines escaped the way JSON service-account files store it.
func generateTestServiceAccountKey(t *testing.T) (escapedPEM string, key *rsa.PrivateKey) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal PKCS8: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return strings.ReplaceAll(string(pemBytes), "\n", `\n`), privateKey
}

func TestBuildFCMJWT(t *testing.T) {
	escapedPEM, key := generateTestServiceAccountKey(t)
	now := time.Unix(1_700_000_000, 0)

	token, err := buildFCMJWT("svc@project.iam.gserviceaccount.com", escapedPEM, now)
	if err != nil {
		t.Fatalf("buildFCMJWT: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT has %d parts, want 3", len(parts))
	}

	var header map[string]string
	rawHeader, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if err := json.Unmarshal(rawHeader, &header); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if header["alg"] != "RS256" || header["typ"] != "JWT" {
		t.Fatalf("unexpected header: %v", header)
	}

	rawClaims, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	var claims fcmClaims
	if err := json.Unmarshal(rawClaims, &claims); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	if claims.Iss != "svc@project.iam.gserviceaccount.com" {
		t.Fatalf("iss = %q", claims.Iss)
	}
	if claims.Scope != fcmScope {
		t.Fatalf("scope = %q, want %q", claims.Scope, fcmScope)
	}
	if claims.Aud != fcmTokenEndpoint {
		t.Fatalf("aud = %q, want %q", claims.Aud, fcmTokenEndpoint)
	}
	if claims.Iat != now.Unix() || claims.Exp != now.Add(time.Hour).Unix() {
		t.Fatalf("iat/exp = %d/%d, want %d/%d",
			claims.Iat, claims.Exp, now.Unix(), now.Add(time.Hour).Unix())
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], sig); err != nil {
		t.Fatalf("signature does not verify: %v", err)
	}

	if _, err := buildFCMJWT("", escapedPEM, now); err == nil {
		t.Fatal("expected an error without a client email")
	}
	if _, err := buildFCMJWT("svc@example.com", "not a pem", now); err == nil {
		t.Fatal("expected an error for an invalid private key")
	}
}

func TestParseServiceAccountKeyPKCS1(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	parsed, err := parseServiceAccountKey(string(pemBytes))
	if err != nil {
		t.Fatalf("parseServiceAccountKey: %v", err)
	}
	if !parsed.Equal(privateKey) {
		t.Fatal("parsed key differs from the generated key")
	}
}

func TestFCMIsGone(t *testing.T) {
	var notFound fcmErrorResponse
	notFound.Error.Status = "NOT_FOUND"

	var unregistered fcmErrorResponse
	_ = json.Unmarshal([]byte(`{"error":{"details":[{"errorCode":"UNREGISTERED"}]}}`), &unregistered)

	tests := []struct {
		name   string
		status int
		apiErr fcmErrorResponse
		want   bool
	}{
		{name: "200", status: http.StatusOK, want: false},
		{name: "404", status: http.StatusNotFound, want: true},
		{name: "410", status: http.StatusGone, want: true},
		{name: "400 not found status", status: http.StatusBadRequest, apiErr: notFound, want: true},
		{name: "400 unregistered detail", status: http.StatusBadRequest, apiErr: unregistered, want: true},
		{name: "500", status: http.StatusInternalServerError, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fcmIsGone(tt.status, tt.apiErr); got != tt.want {
				t.Fatalf("fcmIsGone(%d) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}
