package crypto

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrCosignUnavailable is returned when the cosign CLI is not on PATH. Callers
// treat this as graceful degradation: AgentGuard still runs, it just can't
// cryptographically verify a release artifact's signature on this machine.
var ErrCosignUnavailable = errors.New("crypto: cosign CLI not found on PATH")

// Verifier wraps the sigstore `cosign` command-line tool. Per the tech-stack
// lock (requirements.md §2) we shell out to cosign rather than vendor the
// (large, fast-moving) sigstore Go libraries.
type Verifier struct {
	// Bin is the cosign executable. Defaults to "cosign" (resolved on PATH).
	Bin string
	// lookPath is swappable in tests so we can exercise the unavailable path
	// without mutating the process PATH.
	lookPath func(string) (string, error)
	// run executes the command; swappable in tests.
	run func(ctx context.Context, name string, args ...string) ([]byte, error)
}

// NewVerifier returns a Verifier bound to the cosign binary on PATH.
func NewVerifier() *Verifier {
	return &Verifier{
		Bin:      "cosign",
		lookPath: exec.LookPath,
		run:      defaultRun,
	}
}

func defaultRun(ctx context.Context, name string, args ...string) ([]byte, error) {
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // name is a fixed binary, args are caller-controlled paths
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.Bytes(), err
}

// Available reports whether the cosign CLI can be located. Cheap; callers use
// it to decide whether to attempt verification or skip-with-flag.
func (v *Verifier) Available() bool {
	_, err := v.resolve()
	return err == nil
}

func (v *Verifier) resolve() (string, error) {
	bin := v.Bin
	if bin == "" {
		bin = "cosign"
	}
	lp := v.lookPath
	if lp == nil {
		lp = exec.LookPath
	}
	path, err := lp(bin)
	if err != nil {
		return "", ErrCosignUnavailable
	}
	return path, nil
}

// BlobVerification describes a keyless (Fulcio/Rekor) blob verification, as
// used for AgentGuard's own release artifacts.
type BlobVerification struct {
	// BlobPath is the artifact whose signature we're checking.
	BlobPath string
	// SignaturePath is the detached .sig file.
	SignaturePath string
	// CertPath is the Fulcio certificate (.pem) from keyless signing.
	CertPath string
	// CertIdentity is the expected signer identity (e.g. the GitHub Actions
	// workflow OIDC subject). Empty skips the identity assertion.
	CertIdentity string
	// CertOIDCIssuer is the expected OIDC issuer (e.g.
	// https://token.actions.githubusercontent.com). Empty skips the assertion.
	CertOIDCIssuer string
}

// VerifyBlob runs `cosign verify-blob` for a keyless-signed artifact. It
// returns ErrCosignUnavailable if the CLI is missing, or a wrapped error
// (including cosign's combined output) on a verification failure.
func (v *Verifier) VerifyBlob(ctx context.Context, bv BlobVerification) error {
	bin, err := v.resolve()
	if err != nil {
		return err
	}
	if bv.BlobPath == "" || bv.SignaturePath == "" {
		return errors.New("crypto: VerifyBlob requires BlobPath and SignaturePath")
	}
	args := []string{"verify-blob", "--signature", bv.SignaturePath}
	if bv.CertPath != "" {
		args = append(args, "--certificate", bv.CertPath)
	}
	if bv.CertIdentity != "" {
		args = append(args, "--certificate-identity", bv.CertIdentity)
	}
	if bv.CertOIDCIssuer != "" {
		args = append(args, "--certificate-oidc-issuer", bv.CertOIDCIssuer)
	}
	args = append(args, bv.BlobPath)

	runFn := v.run
	if runFn == nil {
		runFn = defaultRun
	}
	out, err := runFn(ctx, bin, args...)
	if err != nil {
		return fmt.Errorf("crypto: cosign verify-blob failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
