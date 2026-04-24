package cmd

import (
	"strings"
	"testing"
)

// TestVersionCmd_Metadata verifies the `docsiq version` cobra command is
// registered and carries sensible documentation. Detailed version-
// resolution logic lives in internal/buildinfo and is covered there.
func TestVersionCmd_Metadata(t *testing.T) {
	if versionCmd.Use != "version" {
		t.Errorf("Use=%q want version", versionCmd.Use)
	}
	if !strings.Contains(strings.ToLower(versionCmd.Short), "version") {
		t.Errorf("Short=%q does not mention version", versionCmd.Short)
	}
	// The Run func must be non-nil; a nil Run would panic on invocation.
	if versionCmd.Run == nil {
		t.Fatal("versionCmd.Run is nil")
	}
}

// TestVersionCmd_RunDoesNotPanic invokes the command in isolation to
// confirm the full versionInfo → fmt.Printf path executes cleanly.
// Output goes to os.Stdout (Cobra's fmt.Printf path) so we don't
// capture it here — cmd-level tests intentionally stay shallow; the
// stdout shape is covered by the smoke test in the PR verification
// gate.
func TestVersionCmd_RunDoesNotPanic(t *testing.T) {
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("versionCmd.Run panicked: %v", rec)
		}
	}()
	versionCmd.Run(versionCmd, nil)
}
