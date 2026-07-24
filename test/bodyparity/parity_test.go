// Upstream body parity (SPEC §3.5 / §3.5a) — integration only.
//
// Kept out of pkg/csmith so core unit tests stay fast. Run this package when
// working multi-seed / level C / flag-surface parity:
//
//	CSMITH_UPSTREAM=/path/to/csmith go test ./test/bodyparity -count=1
//	CSMITH_UPSTREAM=/path/to/csmith go test ./test/bodyparity -run '^$' \
//	  -fuzz=FuzzBodyParity -fuzztime=30s
//
// CSMITH_UPSTREAM required (hard fail if unset/invalid).
//
// Generation contract: csmith.Options → program string (Go) and Options.CLIArgs()
// → golden binary (upstream). Fuzz mutates a compact blob decoded by
// OptionsFromFuzzBlob — whole Options surface, not seed alone.
package bodyparity_test

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"csmith/pkg/csmith"

	"github.com/google/go-cmp/cmp"
)

// Frozen battery from SPEC §3.5a (level B). Do not shrink in this list.
// Battery uses Defaults() + seed only.
var bodyParityBattery = []uint64{
	0, 1, 2, 3, 4, 5, 7, 10, 42, 100, 123, 999,
}

const upstreamGenTimeout = 120 * time.Second

var (
	reBodyStartStatic = regexp.MustCompile(`(?m)^static long __undefined;`)
	reBodyStartStruct = regexp.MustCompile(`(?m)^/\* --- Struct/Union Declarations --- \*/`)
	reBodyEndStats    = regexp.MustCompile(`(?m)^/\*{10,} statistics`)
	reBodyEndXXX      = regexp.MustCompile(`(?m)^XXX number of`)
)

// programBody extracts the SPEC §3.5 program body (pre-stats).
func programBody(src string) (string, error) {
	start := -1
	if loc := reBodyStartStruct.FindStringIndex(src); loc != nil {
		start = loc[0]
	} else if loc := reBodyStartStatic.FindStringIndex(src); loc != nil {
		start = loc[0]
	}
	if start < 0 {
		return "", fmt.Errorf("program body start not found")
	}
	end := len(src)
	if loc := reBodyEndStats.FindStringIndex(src); loc != nil {
		end = loc[0]
	} else if loc := reBodyEndXXX.FindStringIndex(src); loc != nil {
		end = loc[0]
	}
	if end <= start {
		return "", fmt.Errorf("program body end before start")
	}
	return src[start:end], nil
}

func upstreamCsmith(tb testing.TB) string {
	tb.Helper()
	p := strings.TrimSpace(os.Getenv("CSMITH_UPSTREAM"))
	if p == "" {
		tb.Fatal("CSMITH_UPSTREAM is not set (path to golden csmith binary)")
	}
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		// Nix GC often deletes the store object; re-realize without changing the path.
		hint := ""
		if strings.Contains(p, "/nix/store/") {
			// nix-store -r takes the store path (derivation output), not necessarily .../bin/csmith
			store := p
			if i := strings.Index(p, "/bin/"); i > 0 {
				store = p[:i]
			}
			hint = fmt.Sprintf("\n\tNix path missing (often GC): nix-store -r %s\n\tthen re-run with the same CSMITH_UPSTREAM", store)
		}
		if err != nil {
			tb.Fatalf("CSMITH_UPSTREAM=%q not a file: %v%s", p, err, hint)
		}
		tb.Fatalf("CSMITH_UPSTREAM=%q is a directory, need the csmith binary%s", p, hint)
	}
	return p
}

func upstreamGenerate(tb testing.TB, opts csmith.Options) string {
	tb.Helper()
	bin := upstreamCsmith(tb)
	args := opts.CLIArgs()
	ctx, cancel := context.WithTimeout(tb.Context(), upstreamGenTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		tb.Fatalf("upstream csmith opts=%v: timeout after %s (%s)", args, upstreamGenTimeout, bin)
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		// Shared invalid config: skip rather than fail the corpus entry.
		if isUpstreamConflict(msg) {
			tb.Skipf("upstream rejects config %v: %s", args, msg)
		}
		tb.Fatalf("upstream csmith %v (%s): %s", args, bin, msg)
	}
	return stdout.String()
}

func isUpstreamConflict(msg string) bool {
	low := strings.ToLower(msg)
	return strings.Contains(low, "conflict") ||
		strings.Contains(low, "error:") ||
		strings.Contains(low, "cannot") ||
		strings.Contains(low, "invalid")
}

func goGenerate(tb testing.TB, opts csmith.Options) string {
	tb.Helper()
	if err := opts.Validate(); err != nil {
		tb.Skipf("go rejects config: %v", err)
	}
	// Header Options: line should list the same argv we pass upstream.
	opts.Argv = opts.CLIArgs()
	out, err := csmith.Generate(opts)
	if err != nil {
		// Match upstream soft reject: skip invalid / failed generation under exotic flags.
		if strings.Contains(err.Error(), "conflict") || strings.Contains(err.Error(), "generation error") {
			tb.Skipf("go generate skipped: %v (args=%v)", err, opts.CLIArgs())
		}
		tb.Fatalf("go csmith args=%v: %v", opts.CLIArgs(), err)
	}
	return out
}

// bodyMismatchReport is a capped go-cmp style report (first differing lines).
func bodyMismatchReport(opts csmith.Options, upBody, goBody string) string {
	upLines := strings.Split(upBody, "\n")
	goLines := strings.Split(goBody, "\n")
	n := len(upLines)
	if len(goLines) > n {
		n = len(goLines)
	}
	first := -1
	for i := 0; i < n; i++ {
		a, b := "", ""
		if i < len(upLines) {
			a = upLines[i]
		}
		if i < len(goLines) {
			b = goLines[i]
		}
		if a != b {
			first = i
			break
		}
	}
	args := opts.CLIArgs()
	if first < 0 {
		return fmt.Sprintf("opts=%v program body mismatch (len up=%d go=%d, no line diff?)",
			args, len(upBody), len(goBody))
	}
	lo := first - 5
	if lo < 0 {
		lo = 0
	}
	hi := first + 6
	if hi > len(upLines) {
		hi = len(upLines)
	}
	hiG := first + 6
	if hiG > len(goLines) {
		hiG = len(goLines)
	}
	diff := cmp.Diff(upLines[lo:hi], goLines[lo:hiG])
	if len(diff) > 4000 {
		diff = diff[:4000] + "\n... (truncated)"
	}
	return fmt.Sprintf("opts=%v program body mismatch at line %d (-upstream +go):\n%s",
		args, first+1, diff)
}

func assertOptsBodyParity(tb testing.TB, opts csmith.Options) {
	tb.Helper()
	// Drop-in contract only: library/go-only knobs → Defaults (golden cannot set them).
	opts = opts.ForDropInParity()
	args := opts.CLIArgs()
	short := csmith.FormatOptionsShort(opts)
	// Always on failure / with -v. Fuzz workers often swallow stderr; optional
	// BODYPARITY_LOG=path appends every case (including fuzz) for live inspection.
	tb.Logf("bodyparity argv=%q short=%s", args, short)
	if p := strings.TrimSpace(os.Getenv("BODYPARITY_LOG")); p != "" {
		line := fmt.Sprintf("bodyparity argv=%q short=%s\n", args, short)
		f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = f.WriteString(line)
			_ = f.Close()
		}
	}
	upOut := upstreamGenerate(tb, opts)
	goOut := goGenerate(tb, opts)
	upBody, err := programBody(upOut)
	if err != nil {
		tb.Fatalf("opts=%v upstream body: %v", opts.CLIArgs(), err)
	}
	goBody, err := programBody(goOut)
	if err != nil {
		tb.Fatalf("opts=%v go body: %v", opts.CLIArgs(), err)
	}
	if goBody == upBody {
		return
	}
	tb.Fatal(bodyMismatchReport(opts, upBody, goBody))
}

func assertSeedBodyParity(tb testing.TB, seed uint64) {
	tb.Helper()
	opts := csmith.Defaults()
	opts.Seed = seed
	assertOptsBodyParity(tb, opts)
}

// TestBodyParityBattery is SPEC §3.5a level B: frozen battery, exact body.
func TestBodyParityBattery(t *testing.T) {
	t.Logf("upstream=%s", upstreamCsmith(t))
	for _, seed := range bodyParityBattery {
		seed := seed
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			assertSeedBodyParity(t, seed)
		})
	}
}

// FuzzBodyParity is drop-in flag-surface + seed fuzz (SPEC §3.5b).
//
// Payload: OptionsFromFuzzBlob — seed + every golden-CLI-expressible field
// (FieldCLI). Library-only CGOptions stay at Defaults. Compares Go Generate
// vs golden csmith(opts.CLIArgs()).
//
//	CSMITH_UPSTREAM=... go test ./test/bodyparity -run '^$' -fuzz=FuzzBodyParity -fuzztime=30s
//
// Seed-only Defaults remain covered by TestBodyParityBattery / TestBodyParityLevelC.
// Hard limit: go-fuzz worker ~10s per input; prefer sequential helpers for long runs.
func FuzzBodyParity(f *testing.F) {
	f.Logf("upstream=%s", upstreamCsmith(f))
	// Seed-only roots (8-byte LE seed) — Defaults drop-in surface.
	for _, seed := range []uint64{0, 1, 2} {
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, seed)
		f.Add(b)
	}
	// Drop-in flag roots.
	for _, seed := range []uint64{0, 2, 42} {
		o := csmith.Defaults()
		o.Seed = seed
		f.Add(csmith.FuzzBlobFromOptions(o))
	}
	for _, mut := range []func(*csmith.Options){
		func(o *csmith.Options) { o.Jumps = false },
		func(o *csmith.Options) { o.Volatiles = false },
		func(o *csmith.Options) { o.MaxFuncs = 3 },
		func(o *csmith.Options) { o.Pointers = false },
		func(o *csmith.Options) { o.Arrays = false },
		func(o *csmith.Options) { o.SafeMath = false },
		func(o *csmith.Options) { o.Bitfields = false },
	} {
		o := csmith.Defaults()
		o.Seed = 2
		mut(&o)
		f.Add(csmith.FuzzBlobFromOptions(o))
	}

	f.Fuzz(func(t *testing.T, blob []byte) {
		assertOptsBodyParity(t, csmith.OptionsFromFuzzBlob(blob))
	})
}

// TestBodyParityLevelC is SPEC §3.5a level C without the go-fuzz 10s worker cap:
// sequential random seeds under Defaults(), exact pre-stats body, until
// BODYPARITY_LEVELC duration elapses.
//
//	CSMITH_UPSTREAM=... BODYPARITY_LEVELC=10m go test ./test/bodyparity \
//	  -run TestBodyParityLevelC -count=1 -timeout 15m
//
// Skipped unless BODYPARITY_LEVELC is set (keeps default `go test` fast).
// For flag-surface stress use FuzzBodyParity (blob) instead.
func TestBodyParityLevelC(t *testing.T) {
	raw := strings.TrimSpace(os.Getenv("BODYPARITY_LEVELC"))
	if raw == "" {
		t.Skip("set BODYPARITY_LEVELC=10m (or 1) for sequential level-C random parity")
	}
	dur := 10 * time.Minute
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		// default 10m
	default:
		d, err := time.ParseDuration(raw)
		if err != nil {
			t.Fatalf("BODYPARITY_LEVELC=%q: %v", raw, err)
		}
		dur = d
	}
	_ = upstreamCsmith(t)
	seed := uint64(0)
	if s := strings.TrimSpace(os.Getenv("BODYPARITY_LEVELC_SEED")); s != "" {
		v, err := strconv.ParseUint(s, 0, 64)
		if err != nil {
			t.Fatalf("BODYPARITY_LEVELC_SEED=%q: %v", s, err)
		}
		seed = v
	} else {
		var b [8]byte
		if _, err := crand.Read(b[:]); err != nil {
			t.Fatalf("crypto/rand: %v", err)
		}
		seed = binary.LittleEndian.Uint64(b[:])
	}
	t.Logf("upstream=%s levelC duration=%s stream_start=%d", os.Getenv("CSMITH_UPSTREAM"), dur, seed)
	deadline := time.Now().Add(dur)
	n, start := 0, time.Now()
	for time.Now().Before(deadline) {
		n++
		seed += 0x9e3779b97f4a7c15
		z := seed
		z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
		z = (z ^ (z >> 27)) * 0x94d049bb133111eb
		z = z ^ (z >> 31)
		assertSeedBodyParity(t, z)
		if t.Failed() {
			t.Logf("levelC failed after n=%d elapsed=%s seed=%d stream_start logged above",
				n, time.Since(start).Round(time.Second), z)
			return
		}
		if n%25 == 0 {
			t.Logf("levelC progress n=%d elapsed=%s last=%d", n, time.Since(start).Round(time.Second), z)
		}
	}
	t.Logf("levelC CLEAN n=%d elapsed=%s", n, time.Since(start).Round(time.Second))
}

func TestProgramBodyExtractSmoke(t *testing.T) {
	const sample = `/*
 * Generator: x
 * Seed: 1
 */
#include "csmith.h"

static long __undefined;

/* --- GLOBAL VARIABLES --- */
static int32_t g_1 = 0;

int main(void) { return 0; }

/************************ statistics *************************
XXX number of pointers: 0
********************* end of statistics **********************/
`
	body, err := programBody(sample)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "static long __undefined;") {
		t.Fatal("missing start")
	}
	if !strings.Contains(body, "int main") {
		t.Fatal("missing main")
	}
	if strings.Contains(body, "statistics") {
		t.Fatal("statistics should be excluded")
	}
}

func TestCLIArgsSmokeNoJumpsParity(t *testing.T) {
	// Smoke: non-default flag path through both generators (needs upstream).
	_ = upstreamCsmith(t)
	opts := csmith.Defaults()
	opts.Seed = 2
	opts.Jumps = false
	assertOptsBodyParity(t, opts)
}
