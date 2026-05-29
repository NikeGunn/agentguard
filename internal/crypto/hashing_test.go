package crypto

import (
	"encoding/json"
	"testing"
)

func TestSHA256Hex(t *testing.T) {
	// Known vector: sha256("") and sha256("abc").
	if got := SHA256Hex(nil); got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("empty digest = %s", got)
	}
	if got := SHA256HexString("abc"); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("abc digest = %s", got)
	}
}

func TestCanonicalJSON_KeyOrderIndependent(t *testing.T) {
	a := map[string]any{"b": 1, "a": 2, "nested": map[string]any{"y": 1, "x": 2}}
	b := map[string]any{"a": 2, "nested": map[string]any{"x": 2, "y": 1}, "b": 1}

	ca, err := CanonicalJSON(a)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := CanonicalJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(ca) != string(cb) {
		t.Fatalf("canonical forms differ:\n a=%s\n b=%s", ca, cb)
	}
	// Sanity: it is valid JSON and keys are sorted.
	if string(ca) != `{"a":2,"b":1,"nested":{"x":2,"y":1}}` {
		t.Fatalf("unexpected canonical form: %s", ca)
	}
	var sink any
	if err := json.Unmarshal(ca, &sink); err != nil {
		t.Fatalf("canonical output is not valid JSON: %v", err)
	}
}

func TestCanonicalJSON_StructAndMapAgree(t *testing.T) {
	type s struct {
		Z int `json:"z"`
		A int `json:"a"`
	}
	cs, _ := CanonicalJSON(s{Z: 9, A: 1})
	cm, _ := CanonicalJSON(map[string]any{"a": 1, "z": 9})
	if string(cs) != string(cm) {
		t.Fatalf("struct vs map mismatch: %s != %s", cs, cm)
	}
}

func TestHashToolSchema_StableAcrossKeyOrder(t *testing.T) {
	schema1 := map[string]any{
		"type":       "object",
		"properties": map[string]any{"path": map[string]any{"type": "string"}, "mode": map[string]any{"type": "string"}},
	}
	schema2 := map[string]any{
		"properties": map[string]any{"mode": map[string]any{"type": "string"}, "path": map[string]any{"type": "string"}},
		"type":       "object",
	}
	h1, err := HashToolSchema("read_file", "Read a file", schema1)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := HashToolSchema("read_file", "Read a file", schema2)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("hash unstable across key order: %s != %s", h1, h2)
	}
}

func TestHashToolSchema_DriftChangesHash(t *testing.T) {
	base, _ := HashToolSchema("create_issue", "Create a GitHub issue", nil)
	descDrift, _ := HashToolSchema("create_issue", "Create a GitHub issue AND email it", nil)
	if base == descDrift {
		t.Fatal("description change must change the hash (rug-pull detection)")
	}
}

func TestHashToolList_OrderIndependent(t *testing.T) {
	a := []ToolDef{{Name: "b"}, {Name: "a"}, {Name: "c"}}
	b := []ToolDef{{Name: "c"}, {Name: "a"}, {Name: "b"}}
	ha, err := HashToolList(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := HashToolList(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatalf("tool-list hash depends on order: %s != %s", ha, hb)
	}
	// And it does not mutate the caller's slice.
	if a[0].Name != "b" {
		t.Fatal("HashToolList mutated the input slice")
	}
}
