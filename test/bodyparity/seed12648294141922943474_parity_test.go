package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 12648294141922943474 (LevelC after 1629): Acc pure multi residual free order
// g_177 Acc-early vs Acc-late on parent reads. Open.
func TestSeed12648294141922943474(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 12648294141922943474
	assertOptsBodyParity(t, o)
}
