package registry

import (
	"math"
	"strings"
	"time"
)

// TrustScore returns 0..100 from registry metadata. Components:
//   - age              (older = more trust)        up to 25
//   - popularity       (downloads / stars)         up to 35
//   - license known    (license != "")             10
//   - has repository   (repository != "")          10
//   - has author       (author != "")              5
//   - recent activity  (publishedAt within 1y)     15
//
// nil input returns 0. Multi-source aggregation: average across sources.
func TrustScore(metas ...*Metadata) int {
	if len(metas) == 0 {
		return 0
	}
	var sum float64
	var n int
	for _, m := range metas {
		if m == nil {
			continue
		}
		sum += float64(scoreOne(m))
		n++
	}
	if n == 0 {
		return 0
	}
	return int(math.Round(sum / float64(n)))
}

func scoreOne(m *Metadata) int {
	s := 0.0
	now := time.Now()

	if !m.PublishedAt.IsZero() {
		age := now.Sub(m.PublishedAt) // first-published proxy
		// Cap age contribution at 5y.
		years := math.Min(age.Hours()/24/365, 5)
		s += (years / 5) * 25
		if age < 365*24*time.Hour {
			s += 15 // active in the last year
		}
	}

	switch m.Source {
	case "npm", "pypi":
		// log scale: 100 dl = 0, 10k = ~17, 1m = ~35
		if m.Downloads > 0 {
			s += math.Min(math.Log10(float64(m.Downloads))*7, 35)
		}
	case "github":
		// stars: 10 = ~7, 1k = ~21, 100k = ~35
		if m.Downloads > 0 {
			s += math.Min(math.Log10(float64(m.Downloads)+1)*7, 35)
		}
	}

	if strings.TrimSpace(m.License) != "" {
		s += 10
	}
	if strings.TrimSpace(m.Repository) != "" {
		s += 10
	}
	if strings.TrimSpace(m.Author) != "" {
		s += 5
	}

	if s > 100 {
		s = 100
	}
	if s < 0 {
		s = 0
	}
	return int(math.Round(s))
}
