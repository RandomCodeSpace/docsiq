package hookinstaller

import (
	"fmt"
	"sort"
)

// All returns one Installer per supported client. Callers typically
// iterate this list for the "install/uninstall everything" path.
// The slice order is stable so CLI output is deterministic.
func All() []Installer {
	return []Installer{
		ClaudeInstaller{},
		CursorInstaller{},
		CopilotInstaller{},
		CodexInstaller{},
	}
}

// Names returns the sorted list of supported client names. Useful for
// flag validation and help text.
func Names() []string {
	out := make([]string, 0, len(All()))
	for _, i := range All() {
		out = append(out, i.Name())
	}
	sort.Strings(out)
	return out
}

// ByName returns the Installer for name, or an error if the name is not
// one of the supported clients. Names are lowercase.
func ByName(name string) (Installer, error) {
	for _, i := range All() {
		if i.Name() == name {
			return i, nil
		}
	}
	return nil, fmt.Errorf("unknown client %q (supported: %v)", name, Names())
}
