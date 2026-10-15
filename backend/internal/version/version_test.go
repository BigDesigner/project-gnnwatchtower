package version

import (
	"testing"
)

func TestGet(t *testing.T) {
	v := Get()
	if v == "" {
		t.Fatalf("expected non-empty version, got empty string")
	}
	if v != "0.4.2" {
		t.Logf("Notice: Active version resolved to: %s", v)
	}
}
