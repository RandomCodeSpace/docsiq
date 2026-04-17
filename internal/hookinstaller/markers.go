package hookinstaller

import "strings"

// looksLikeDocsiqHook matches a command string that appears to invoke
// our installed hook. Used by the Uninstall path when the exact install
// location isn't remembered. We deliberately err on the side of matching
// (remove anything looking like a docsiq hook) because a false positive
// only un-registers a hook — the script file itself is left on disk.
func looksLikeDocsiqHook(cmd string) bool {
	if cmd == "" {
		return false
	}
	// Most common install locations:
	//   ~/.docsiq/hooks/hook.sh
	//   $DATA_DIR/hooks/hook.sh
	markers := []string{
		"/docsiq/hooks/hook.sh",
		"/.docsiq/hooks/hook.sh",
		"docsiq-hook.sh",
	}
	for _, m := range markers {
		if strings.Contains(cmd, m) {
			return true
		}
	}
	return false
}
