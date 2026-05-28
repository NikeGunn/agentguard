package registry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTrustScoreEmpty(t *testing.T) {
	require.Equal(t, 0, TrustScore())
	require.Equal(t, 0, TrustScore(nil))
}

func TestTrustScoreHighSignal(t *testing.T) {
	m := &Metadata{
		Source:      "npm",
		Name:        "react",
		Version:     "18",
		Downloads:   20_000_000,
		PublishedAt: time.Now().Add(-3 * 365 * 24 * time.Hour),
		License:     "MIT",
		Repository:  "https://github.com/facebook/react",
		Author:      "fb",
	}
	require.GreaterOrEqual(t, TrustScore(m), 70)
}

func TestTrustScoreLowSignal(t *testing.T) {
	m := &Metadata{Source: "npm", Name: "obscure", Downloads: 2}
	require.Less(t, TrustScore(m), 30)
}
