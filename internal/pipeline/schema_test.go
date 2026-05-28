package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSchemaValidatorAcceptsRequest(t *testing.T) {
	r := SchemaValidator{}.Run(context.Background(), &Message{
		Direction: Outbound,
		Raw:       []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call"}`),
	})
	require.Equal(t, VerdictPass, r.Verdict)
}

func TestSchemaValidatorAcceptsResponse(t *testing.T) {
	r := SchemaValidator{}.Run(context.Background(), &Message{
		Direction: Inbound,
		Raw:       []byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`),
	})
	require.Equal(t, VerdictPass, r.Verdict)
}

func TestSchemaValidatorRejectsBadJSON(t *testing.T) {
	r := SchemaValidator{}.Run(context.Background(), &Message{
		Direction: Outbound,
		Raw:       []byte(`{not json`),
	})
	require.Equal(t, VerdictBlock, r.Verdict)
	require.Equal(t, "invalid_json", r.Reason)
}

func TestSchemaValidatorRejectsBadVersion(t *testing.T) {
	r := SchemaValidator{}.Run(context.Background(), &Message{
		Direction: Outbound,
		Raw:       []byte(`{"jsonrpc":"1.0","id":1,"method":"x"}`),
	})
	require.Equal(t, VerdictBlock, r.Verdict)
	require.Equal(t, "bad_jsonrpc_version", r.Reason)
}

func TestSchemaValidatorRejectsOutboundWithoutMethod(t *testing.T) {
	r := SchemaValidator{}.Run(context.Background(), &Message{
		Direction: Outbound,
		Raw:       []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`),
	})
	require.Equal(t, VerdictBlock, r.Verdict)
	require.Equal(t, "outbound_without_method", r.Reason)
}

func TestSchemaValidatorRejectsResultAndError(t *testing.T) {
	r := SchemaValidator{}.Run(context.Background(), &Message{
		Direction: Inbound,
		Raw:       []byte(`{"jsonrpc":"2.0","id":1,"result":{},"error":{"code":1,"message":"x"}}`),
	})
	require.Equal(t, VerdictBlock, r.Verdict)
}
