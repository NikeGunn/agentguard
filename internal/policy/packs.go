package policy

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
)

//go:embed builtin/*.yaml
var builtinPacks embed.FS

// LoadBuiltin returns the compiled rules from one of the binary-embedded
// builtin packs. Recognised names today: "default", "strict".
func LoadBuiltin(name string) (*PackFile, []Rule, error) {
	data, err := builtinPacks.ReadFile(path.Join("builtin", name+".yaml"))
	if err != nil {
		return nil, nil, fmt.Errorf("load builtin pack %q: %w", name, err)
	}
	return CompilePack(data)
}

// ListBuiltin returns the slugs of all packs embedded in the binary, sorted.
func ListBuiltin() ([]string, error) {
	entries, err := fs.ReadDir(builtinPacks, "builtin")
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if len(name) > 5 && name[len(name)-5:] == ".yaml" {
			out = append(out, name[:len(name)-5])
		}
	}
	sort.Strings(out)
	return out, nil
}
