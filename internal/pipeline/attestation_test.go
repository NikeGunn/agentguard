package pipeline

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type memAttStore struct {
	mu sync.Mutex
	m  map[string]string
}

func newMemAttStore() *memAttStore { return &memAttStore{m: map[string]string{}} }

func (s *memAttStore) GetServerHash(_ context.Context, id string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[id]
	return v, ok, nil
}
func (s *memAttStore) SetServerHash(_ context.Context, id, h string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[id] = h
	return nil
}

func TestAttestationFirstSeenPasses(t *testing.T) {
	st := NewAttestationStage(newMemAttStore())
	m := &Message{
		Direction: Inbound, Method: "tools/list", ServerName: "github-mcp",
		Raw: []byte(`{"result":{"tools":[{"name":"a","description":"x"}]}}`),
	}
	r := st.Run(context.Background(), m)
	require.Equal(t, VerdictPass, r.Verdict)
	require.Equal(t, "first seen", r.Reason)
}

func TestAttestationDriftFlags(t *testing.T) {
	st := NewAttestationStage(newMemAttStore())
	m1 := &Message{
		Direction: Inbound, Method: "tools/list", ServerName: "s",
		Raw: []byte(`{"result":{"tools":[{"name":"a","description":"v1"}]}}`),
	}
	st.Run(context.Background(), m1)
	m2 := &Message{
		Direction: Inbound, Method: "tools/list", ServerName: "s",
		Raw: []byte(`{"result":{"tools":[{"name":"a","description":"v2"}]}}`),
	}
	r := st.Run(context.Background(), m2)
	require.Equal(t, VerdictFlag, r.Verdict)
	require.Contains(t, r.Detail, "old_hash")
	require.Contains(t, r.Detail, "new_hash")
}

func TestAttestationStableUnderReorder(t *testing.T) {
	a := []byte(`{"result":{"tools":[{"name":"a"},{"name":"b"}]}}`)
	b := []byte(`{"result":{"tools":[{"name":"b"},{"name":"a"}]}}`)
	ha, err := hashToolsList(a)
	require.NoError(t, err)
	hb, err := hashToolsList(b)
	require.NoError(t, err)
	require.Equal(t, ha, hb)
}

func TestAttestationSkipsNonInbound(t *testing.T) {
	st := NewAttestationStage(newMemAttStore())
	r := st.Run(context.Background(), &Message{Direction: Outbound, Method: "tools/list"})
	require.Equal(t, VerdictPass, r.Verdict)
}
