package hookinstaller

import _ "embed"

// HookScript is the POSIX shell SessionStart hook. It ships embedded in
// the binary so `docsiq hooks install` never has to shell out to locate
// a source tree. See assets/hook.sh for the script itself.
//
//go:embed assets/hook.sh
var HookScript []byte
