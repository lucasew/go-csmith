package bodyparity_test

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"csmith/pkg/csmith"
)

func TestCampFails(t *testing.T) {
	_ = upstreamCsmith(t)
	dir := "testdata/campfails"
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".blob.hex") {
			continue
		}
		e := e
		t.Run(e.Name(), func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			b, err := hex.DecodeString(strings.TrimSpace(string(raw)))
			if err != nil {
				t.Fatal(err)
			}
			assertOptsBodyParity(t, csmith.OptionsFromFuzzBlob(b))
		})
	}
}
