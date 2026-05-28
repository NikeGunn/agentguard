// Package ml implements AgentGuard's lightweight prompt-injection
// classifier. We deliberately avoid pulling an ONNX runtime (CGO,
// hundreds of MB) and instead score with a calibrated feature set
// that detects the long tail of injection patterns the regex scanner
// can't enumerate.
package ml

import (
	"crypto/sha256"
	"math"
	"strings"
	"sync"
	"unicode"
)

// Classification is the output of Classify.
type Classification struct {
	Confidence float64
	Label      string
	Features   []string
}

const (
	FlagThreshold  = 0.50
	BlockThreshold = 0.85
)

type Classifier struct {
	mu    sync.RWMutex
	cache map[[16]byte]Classification
	max   int
}

func New(capacity int) *Classifier {
	if capacity <= 0 {
		capacity = 4096
	}
	return &Classifier{cache: make(map[[16]byte]Classification, capacity), max: capacity}
}

func (c *Classifier) Classify(text string) Classification {
	if text == "" {
		return Classification{Label: "benign"}
	}
	key := hashKey(text)
	c.mu.RLock()
	if v, ok := c.cache[key]; ok {
		c.mu.RUnlock()
		return v
	}
	c.mu.RUnlock()

	out := score(text)

	c.mu.Lock()
	if len(c.cache) >= c.max {
		for k := range c.cache {
			delete(c.cache, k)
			break
		}
	}
	c.cache[key] = out
	c.mu.Unlock()
	return out
}

func hashKey(s string) [16]byte {
	sum := sha256.Sum256([]byte(s))
	var k [16]byte
	copy(k[:], sum[:16])
	return k
}

type Feature struct {
	Name   string
	Weight float64
	Match  func(lower string) bool
}

var features = []Feature{
	{"ignore_previous", 0.55, func(s string) bool {
		return strings.Contains(s, "ignore previous") ||
			strings.Contains(s, "ignore the above") ||
			strings.Contains(s, "disregard previous") ||
			strings.Contains(s, "forget previous")
	}},
	{"system_role_takeover", 0.55, func(s string) bool {
		return strings.Contains(s, "you are now") ||
			(strings.Contains(s, "act as") && strings.Contains(s, "admin")) ||
			strings.Contains(s, "system:") ||
			strings.Contains(s, "<|system|>") ||
			strings.Contains(s, "[system]")
	}},
	{"role_dan", 0.65, func(s string) bool {
		return strings.Contains(s, "do anything now") ||
			(strings.Contains(s, "dan") && strings.Contains(s, "jailbreak")) ||
			strings.Contains(s, "developer mode")
	}},
	{"policy_override", 0.5, func(s string) bool {
		return strings.Contains(s, "without restrictions") ||
			strings.Contains(s, "override safety") ||
			strings.Contains(s, "bypass policy") ||
			strings.Contains(s, "no ethical")
	}},
	{"exfil_marker", 0.4, func(s string) bool {
		return strings.Contains(s, "send to http") ||
			strings.Contains(s, "exfiltrate") ||
			strings.Contains(s, "post to webhook") ||
			(strings.Contains(s, "leak the") && strings.Contains(s, "key"))
	}},
	{"reveal_secrets", 0.5, func(s string) bool {
		return strings.Contains(s, "reveal your") ||
			strings.Contains(s, "print your prompt") ||
			strings.Contains(s, "show me your system") ||
			strings.Contains(s, "what are your instructions")
	}},
	{"encoded_payload", 0.35, func(s string) bool {
		return strings.Contains(s, "base64:") ||
			strings.Contains(s, "rot13:") ||
			strings.Contains(s, "fromcharcode")
	}},
	{"prompt_injection_marker", 0.4, func(s string) bool {
		return (strings.Contains(s, "{{") && strings.Contains(s, "}}")) ||
			strings.Contains(s, "${jndi:") ||
			strings.Contains(s, "<!--injection") ||
			strings.Contains(s, "[[inject")
	}},
	{"destructive_shell", 0.4, func(s string) bool {
		return strings.Contains(s, "rm -rf /") ||
			strings.Contains(s, "del /f /s /q c:") ||
			strings.Contains(s, ":(){:|:&};:") ||
			strings.Contains(s, "drop table") ||
			strings.Contains(s, "truncate table")
	}},
	{"html_script", 0.25, func(s string) bool {
		return strings.Contains(s, "<script") ||
			strings.Contains(s, "onerror=") ||
			strings.Contains(s, "javascript:")
	}},
}

func score(text string) Classification {
	lower := strings.ToLower(text)
	var hits []string
	raw := 0.0
	for _, f := range features {
		if f.Match(lower) {
			raw += f.Weight
			hits = append(hits, f.Name)
		}
	}
	if containsZeroWidth(text) {
		raw += 0.25
		hits = append(hits, "zero_width_chars")
	}
	if len(text) > 4000 && lowAlphaRatio(text) < 0.4 {
		raw += 0.15
		hits = append(hits, "low_alpha_long_input")
	}
	conf := logistic(raw)
	label := "benign"
	switch {
	case conf >= BlockThreshold:
		label = "injection"
	case conf >= FlagThreshold:
		label = "suspicious"
	}
	return Classification{Confidence: conf, Label: label, Features: hits}
}

func logistic(raw float64) float64 {
	x := raw*3 - 1.5
	return 1.0 / (1.0 + math.Exp(-x))
}

func containsZeroWidth(s string) bool {
	for _, r := range s {
		switch r {
		case 0x200B, 0x200C, 0x200D, 0xFEFF, 0x2060:
			return true
		}
	}
	return false
}
func lowAlphaRatio(s string) float64 {
	if s == "" {
		return 1
	}
	alpha, total := 0, 0
	for _, r := range s {
		total++
		if unicode.IsLetter(r) {
			alpha++
		}
	}
	return float64(alpha) / float64(total)
}

func (c *Classifier) CacheStats() (int, int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache), c.max
}
