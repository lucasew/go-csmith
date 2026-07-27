package bodyparity_test

import (
	"testing"
	"csmith/pkg/csmith"
)

// Seed 12336: pure multi-prefix Acc-late (g_659) — multi-prefix split early+late.
func TestSeed12336(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 12336
	assertOptsBodyParity(t, o)
}
