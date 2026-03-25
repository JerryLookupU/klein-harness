package cli

import (
	"fmt"
	"path/filepath"
)

func ResolveRootArg(args []string) (string, []string, error) {
	root := "."
	rest := args
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		root = args[0]
		rest = args[1:]
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", nil, fmt.Errorf("resolve root: %w", err)
	}
	return absRoot, rest, nil
}
