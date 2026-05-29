package ml

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func writeModel(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "model.onnx")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadModel_HashesAndCaches(t *testing.T) {
	path := writeModel(t, "fake-onnx-bytes")
	want := sha256.Sum256([]byte("fake-onnx-bytes"))
	wantHex := hex.EncodeToString(want[:])

	l := NewLoader()
	mi, err := l.LoadModel(path)
	if err != nil {
		t.Fatal(err)
	}
	if !mi.Present() {
		t.Fatal("model should be present")
	}
	if mi.SHA256 != wantHex {
		t.Fatalf("sha256 = %s, want %s", mi.SHA256, wantHex)
	}
	if mi.Size != int64(len("fake-onnx-bytes")) {
		t.Fatalf("size = %d", mi.Size)
	}

	// Second load is served from cache (same value).
	mi2, err := l.LoadModel(path)
	if err != nil || mi2.SHA256 != mi.SHA256 {
		t.Fatalf("cached load mismatch: %v %s", err, mi2.SHA256)
	}
}

func TestLoadModel_MissingReturnsSentinel(t *testing.T) {
	l := NewLoader()
	mi, err := l.LoadModel(filepath.Join(t.TempDir(), "absent.onnx"))
	if err != ErrModelNotFound {
		t.Fatalf("want ErrModelNotFound, got %v", err)
	}
	if mi.Present() {
		t.Fatal("missing model must not report Present")
	}
}

func TestVerify(t *testing.T) {
	path := writeModel(t, "weights")
	sum := sha256.Sum256([]byte("weights"))
	good := hex.EncodeToString(sum[:])

	l := NewLoader()
	if err := l.Verify(path, good); err != nil {
		t.Fatalf("Verify with correct hash failed: %v", err)
	}
	if err := l.Verify(path, "deadbeef"); err == nil {
		t.Fatal("Verify with wrong hash should fail")
	}
	if err := l.Verify(filepath.Join(t.TempDir(), "nope.onnx"), good); err != ErrModelNotFound {
		t.Fatalf("Verify of missing model = %v, want ErrModelNotFound", err)
	}
}

func TestDefaultModelPath(t *testing.T) {
	p, err := DefaultModelPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != DefaultModelName {
		t.Fatalf("default path basename = %s", filepath.Base(p))
	}
}
