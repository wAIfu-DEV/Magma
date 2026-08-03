package types

import "fmt"

// ResolveModulePrefix resolves the leading module namespace in a dotted name.
// The first component is a local import; every subsequent component traversed
// as a module must have been publicly re-exported by the preceding module.
// It returns the target module and the number of consumed name components.
func ResolveModulePrefix(modules map[string]*NodeGlobal, owner *NodeGlobal, parts []string) (string, int, error) {
	if owner == nil || len(parts) == 0 {
		return "", 0, fmt.Errorf("missing qualified name")
	}
	target, ok := owner.ImportAlias[parts[0]]
	if !ok {
		return "", 0, fmt.Errorf("unknown module alias '%s'", parts[0])
	}
	consumed := 1
	seen := map[string]bool{target: true}
	for consumed < len(parts)-1 {
		module := modules[target]
		alias := parts[consumed]
		if module == nil || !module.PublicImportAlias[alias] {
			break
		}
		next, ok := module.ImportAlias[alias]
		if !ok {
			return "", 0, fmt.Errorf("public module alias '%s' has no target", alias)
		}
		if seen[next] {
			return "", 0, fmt.Errorf("public module re-export cycle through '%s'", alias)
		}
		seen[next] = true
		target = next
		consumed++
	}
	return target, consumed, nil
}
