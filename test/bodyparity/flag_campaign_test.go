package bodyparity_test

import (
	crand "crypto/rand"
	"fmt"
	"os"
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
	n := 0
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
		n++
		// Subtest so Skip (upstream conflict) does not abort the whole campaign.
		name := fmt.Sprintf("n%d_s%d", n, opts.Seed)
		ok := t.Run(name, func(t *testing.T) {
			assertOptsBodyParity(t, opts)
		})
		if !ok && t.Failed() {
			// Hard fail already recorded; stop so the crasher is the campaign exit.
			return
		}
		if n%10 == 0 {
			t.Logf("flagcamp n=%d last=%s", n, csmith.FormatOptionsShort(opts.ForDropInParity()))
		}
	}
	t.Logf("flagcamp CLEAN n=%d elapsed=%s", n, dur)
}
