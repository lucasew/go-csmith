package bodyparity_test

import (
	"testing"

	"csmith/pkg/csmith"
)

// Regression: mid-gen ArrayAccess cache froze ACCESS_ONCE before isAddrTaken.
func TestCrestAccessOnceSeed1(t *testing.T) {
	_ = upstreamCsmith(t)
	o := csmith.Defaults()
	o.Seed = 1
	o.AccessOnce = true
	o.Crest = true
	assertOptsBodyParity(t, o)
}
