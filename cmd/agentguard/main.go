// Command agentguard is the single-binary entrypoint for the AgentGuard
// sidecar. All real logic lives in internal packages; this is just the cobra
// root.
package main

import (
	"fmt"
	"os"

	"github.com/agentguard/agentguard/internal/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "agentguard:", err)
		os.Exit(1)
	}
}
