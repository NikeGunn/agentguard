package crypto

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestVerifier_Unavailable(t *testing.T) {
	v := &Verifier{
		Bin:      "cosign",
		lookPath: func(string) (string, error) { return "", errors.New("not found") },
	}
	if v.Available() {
		t.Fatal("Available should be false when lookPath fails")
	}
	err := v.VerifyBlob(context.Background(), BlobVerification{BlobPath: "a", SignaturePath: "a.sig"})
	if !errors.Is(err, ErrCosignUnavailable) {
		t.Fatalf("want ErrCosignUnavailable, got %v", err)
	}
}

func TestVerifier_Available(t *testing.T) {
	v := &Verifier{
		Bin:      "cosign",
		lookPath: func(string) (string, error) { return "/usr/local/bin/cosign", nil },
	}
	if !v.Available() {
		t.Fatal("Available should be true when lookPath succeeds")
	}
}

func TestVerifier_VerifyBlob_BuildsArgsAndSucceeds(t *testing.T) {
	var gotArgs []string
	v := &Verifier{
		Bin:      "cosign",
		lookPath: func(string) (string, error) { return "cosign", nil },
		run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			gotArgs = args
			return []byte("Verified OK"), nil
		},
	}
	err := v.VerifyBlob(context.Background(), BlobVerification{
		BlobPath:       "agentguard_linux_amd64",
		SignaturePath:  "agentguard_linux_amd64.sig",
		CertPath:       "cert.pem",
		CertIdentity:   "https://github.com/agentguard/agentguard/.github/workflows/release.yml@refs/tags/v1",
		CertOIDCIssuer: "https://token.actions.githubusercontent.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	joined := strings.Join(gotArgs, " ")
	for _, want := range []string{
		"verify-blob",
		"--signature agentguard_linux_amd64.sig",
		"--certificate cert.pem",
		"--certificate-identity ",
		"--certificate-oidc-issuer https://token.actions.githubusercontent.com",
		"agentguard_linux_amd64",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q; got: %s", want, joined)
		}
	}
	// Blob path must be the final positional argument.
	if gotArgs[len(gotArgs)-1] != "agentguard_linux_amd64" {
		t.Errorf("blob path must be last arg, got %s", gotArgs[len(gotArgs)-1])
	}
}

func TestVerifier_VerifyBlob_PropagatesFailure(t *testing.T) {
	v := &Verifier{
		Bin:      "cosign",
		lookPath: func(string) (string, error) { return "cosign", nil },
		run: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("signature mismatch"), &exec.ExitError{}
		},
	}
	err := v.VerifyBlob(context.Background(), BlobVerification{BlobPath: "a", SignaturePath: "a.sig"})
	if err == nil || !strings.Contains(err.Error(), "signature mismatch") {
		t.Fatalf("want wrapped failure with cosign output, got %v", err)
	}
}

func TestVerifier_VerifyBlob_RequiresPaths(t *testing.T) {
	v := &Verifier{lookPath: func(string) (string, error) { return "cosign", nil }}
	if err := v.VerifyBlob(context.Background(), BlobVerification{}); err == nil {
		t.Fatal("expected error when BlobPath/SignaturePath empty")
	}
}
