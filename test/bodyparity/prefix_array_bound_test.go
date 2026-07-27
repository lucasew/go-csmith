package bodyparity_test

import (
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"csmith/pkg/csmith"
)

// ArrayVariable::OutputLower/UpperBound use bare name (not get_actual_name).
// Under --prefix-name, subject empties but &g_N[0] bounds remain (n12 campfail).
func TestPrefixNameArrayBoundAssert(t *testing.T) {
	_ = upstreamCsmith(t)
	raw, err := os.ReadFile("testdata/campfails/n12_s12763858664237354365.blob.hex")
	if err != nil {
		t.Fatal(err)
	}
	b, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	assertOptsBodyParity(t, csmith.OptionsFromFuzzBlob(b))
}
