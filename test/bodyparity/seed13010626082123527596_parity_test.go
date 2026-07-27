package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Seed 13010626082123527596 (LevelC): FE g_529.f2 Acc-early vs Acc-late (UP late after
// g_609 g_78 g_766). Open. Session/FM-local — no package mutable state.
func TestSeed13010626082123527596(t *testing.T) {
	o := csmith.Defaults()
	o.Seed = 13010626082123527596
	assertOptsBodyParity(t, o)
}
