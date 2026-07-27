package bodyparity_test

import (
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"csmith/pkg/csmith"
)

func TestBuiltinsRandomRandomParity(t *testing.T) {
	raw, err := os.ReadFile("testdata/campfails/n37_s7289571337067984862.blob.hex")
	if err != nil {
		t.Fatal(err)
	}
	b, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	opts := csmith.OptionsFromFuzzBlob(b)
	t.Logf("CLI=%v", opts.ForDropInParity().CLIArgs())
	t.Logf("short=%s", csmith.FormatOptionsShort(opts.ForDropInParity()))
	assertOptsBodyParity(t, opts)
}
