//go:build scale

package notes

import (
	"fmt"
	"testing"
)

// TestScale_1000Notes writes 1000 notes across 10 buckets and verifies
// the key listing. Gated behind the `scale` build tag so default PR CI
// stays fast; the nightly workflow runs it via `-tags "sqlite_fts5 scale"`.
func TestScale_1000Notes(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 1000; i++ {
		k := fmt.Sprintf("bucket%d/note%d", i%10, i)
		if err := Write(dir, &Note{Key: k, Content: "x"}); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	keys, err := ListKeys(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1000 {
		t.Errorf("expected 1000, got %d", len(keys))
	}
}
