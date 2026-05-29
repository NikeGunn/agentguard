package ml

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// ErrModelNotFound is returned by LoadModel when the model file is absent. The
// classifier degrades gracefully (requirements.md §4.6): with no model loaded
// AgentGuard still runs the feature-based Classifier, it just can't run the
// ONNX path. `agentguard doctor` reports model presence + checksum using this
// package.
var ErrModelNotFound = errors.New("ml: model file not found")

// ModelInfo describes a model file on disk: its path, byte size, and SHA-256.
// doctor prints the checksum so a user can confirm the shipped model wasn't
// tampered with.
type ModelInfo struct {
	Path   string
	Size   int64
	SHA256 string
	loaded bool // true once the bytes have been read+hashed
}

// Present reports whether the model file exists and was successfully hashed.
func (m ModelInfo) Present() bool { return m.loaded }

// DefaultModelName is the file the release ships (requirements.md §1).
const DefaultModelName = "injection-classifier-v1.onnx"

// DefaultModelPath returns the canonical model location:
// ~/.agentguard/models/<DefaultModelName>. It does not check existence.
func DefaultModelPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agentguard", "models", DefaultModelName), nil
}

// Loader loads and caches ModelInfo by path. The model bytes themselves aren't
// retained — only the metadata — because the feature-based Classifier doesn't
// need the ONNX weights in memory; the cache avoids re-hashing a multi-MB file
// on every doctor run or daemon restart.
type Loader struct {
	mu    sync.Mutex
	cache map[string]ModelInfo
}

// NewLoader returns an empty Loader.
func NewLoader() *Loader { return &Loader{cache: make(map[string]ModelInfo)} }

// LoadModel returns metadata for the model at path, hashing it on first load
// and serving the cached result thereafter. A missing file yields
// ErrModelNotFound (not a generic os error) so callers can branch on graceful
// degradation cleanly.
func (l *Loader) LoadModel(path string) (ModelInfo, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if mi, ok := l.cache[path]; ok {
		return mi, nil
	}

	f, err := os.Open(path) //nolint:gosec // path is an operator-supplied model location
	if err != nil {
		if os.IsNotExist(err) {
			return ModelInfo{Path: path}, ErrModelNotFound
		}
		return ModelInfo{Path: path}, err
	}
	defer func() { _ = f.Close() }()

	stat, err := f.Stat()
	if err != nil {
		return ModelInfo{Path: path}, err
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ModelInfo{Path: path}, err
	}
	mi := ModelInfo{
		Path:   path,
		Size:   stat.Size(),
		SHA256: hex.EncodeToString(h.Sum(nil)),
		loaded: true,
	}
	l.cache[path] = mi
	return mi, nil
}

// Verify checks that the model at path matches an expected SHA-256 (lowercase
// hex). Returns ErrModelNotFound if absent, or a mismatch error otherwise.
func (l *Loader) Verify(path, wantSHA256 string) error {
	mi, err := l.LoadModel(path)
	if err != nil {
		return err
	}
	if mi.SHA256 != wantSHA256 {
		return errors.New("ml: model checksum mismatch: have " + mi.SHA256 + " want " + wantSHA256)
	}
	return nil
}
