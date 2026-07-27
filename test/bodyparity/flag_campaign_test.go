package bodyparity_test

import (
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"csmith/pkg/csmith"
)

// Manual campaign: random fuzz blobs under Defaults surface for duration.
// Not go-fuzz — avoids 10s worker false-hangs on large programs.
func TestBodyParityFlagCampaign(t *testing.T) {
	raw := os.Getenv("BODYPARITY_FLAGCAMP")
	if raw == "" {
		t.Skip("set BODYPARITY_FLAGCAMP=2m")
	}
	dur, err := time.ParseDuration(raw)
	if err != nil {
		t.Fatal(err)
	}
	_ = upstreamCsmith(t)
	deadline := time.Now().Add(dur)
	n, compared := 0, 0
	// Start from a valid v2 drop-in blob size, then re-randomize payload.
	// Random bytes without magic 0xFF are seed-only (legacy); always use v2.
	base := csmith.FuzzBlobFromOptions(csmith.Defaults())
	for time.Now().Before(deadline) {
		blob := make([]byte, len(base))
		copy(blob, base)
		if _, err := crand.Read(blob[2:]); err != nil { // keep magic/version at [0:2]
			t.Fatal(err)
		}
		// rare seed-only stress
		var coin [1]byte
		_, _ = crand.Read(coin[:])
		if coin[0]%5 == 0 {
			var seedb [8]byte
			_, _ = crand.Read(seedb[:])
			blob = seedb[:]
		}
		opts := csmith.OptionsFromFuzzBlob(blob)
		// DFS exhaustive burns the campaign budget; re-roll to random-mode.
		if opts.DFSExhaustive {
			opts.DFSExhaustive = false
			opts.RandomBased = true
		}
		n++
		// Subtest so Skip (upstream conflict) does not abort the whole campaign.
		name := fmt.Sprintf("n%d_s%d", n, opts.Seed)
		ok := t.Run(name, func(t *testing.T) {
			assertOptsBodyParity(t, opts)
		})
		if !ok && t.Failed() {
			// Persist blob for exact repro (CLIArgs alone can lose drop-in detail).
			dumpCampBlob(t, n, opts)
			// Hard fail already recorded; stop so the crasher is the campaign exit.
			return
		}
		if ok {
			compared++
		}
		if n%10 == 0 {
			t.Logf("flagcamp n=%d compared=%d last=%s", n, compared, csmith.FormatOptionsShort(opts.ForDropInParity()))
		}
	}
	t.Logf("flagcamp CLEAN n=%d compared=%d elapsed=%s", n, compared, dur)
}

// dumpCampBlob writes FuzzBlobFromOptions(ForDropInParity) hex under testdata/campfails/.
func dumpCampBlob(t *testing.T, n int, opts csmith.Options) {
	t.Helper()
	o := opts.ForDropInParity()
	blob := csmith.FuzzBlobFromOptions(o)
	dir := filepath.Join("testdata", "campfails")
	_ = os.MkdirAll(dir, 0o755)
	name := fmt.Sprintf("n%d_s%d.blob.hex", n, o.Seed)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(hex.EncodeToString(blob)+"\n"), 0o644); err != nil {
		t.Logf("camp blob dump failed: %v", err)
		return
	}
	t.Logf("camp DIFF blob hex written %s (%d bytes payload)", path, len(blob))
}
