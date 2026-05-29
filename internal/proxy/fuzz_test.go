package proxy

import "testing"

// FuzzParseRPC asserts the JSON-RPC envelope parser never panics on arbitrary
// bytes — it is fed untrusted request bodies on the HTTP transport. toolName()
// is exercised on the result so the params sub-parse is fuzzed too.
func FuzzParseRPC(f *testing.F) {
	seeds := []string{
		``,
		`{}`,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"fs.read"}}`,
		`{"jsonrpc":"2.0","method":"tools/call","params":{"name":123}}`,
		`{"method":"tools/call","params":"not-an-object"}`,
		`   {"jsonrpc":"2.0"}   `,
		`[1,2,3]`,
		`null`,
		`{"params":{`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(_ *testing.T, b []byte) {
		frame := parseRPC(b)
		_ = frame.toolName()
	})
}
