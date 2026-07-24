// Upstream body parity (SPEC §3.5 / §3.5a) — integration only.
//
// Kept out of pkg/csmith so core unit tests stay fast. Run this package when
// working multi-seed / level C:
//
//	CSMITH_UPSTREAM=/path/to/csmith go test ./test/bodyparity -count=1
//	CSMITH_UPSTREAM=/path/to/csmith go test ./test/bodyparity -run '^$' \
//	  -fuzz=FuzzBodyParity -fuzztime=30s
//
// CSMITH_UPSTREAM required (hard fail if unset/invalid).
package bodyparity_test

import (
	"bytes"
	"context"
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
		tb.Fatalf("CSMITH_UPSTREAM=%q not a file", p)
	}
	return p
}

func upstreamGenerate(tb testing.TB, seed uint64) string {
	tb.Helper()
	bin := upstreamCsmith(tb)
	ctx, cancel := context.WithTimeout(tb.Context(), upstreamGenTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-s", strconv.FormatUint(seed, 10))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		tb.Fatalf("upstream csmith seed=%d: timeout after %s (%s)", seed, upstreamGenTimeout, bin)
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		tb.Fatalf("upstream csmith seed=%d (%s): %s", seed, bin, msg)
	}
	return stdout.String()
}

func goGenerate(tb testing.TB, seed uint64) string {
	tb.Helper()
	opts := csmith.Defaults()
	opts.Seed = seed
	out, err := csmith.Generate(opts)
	if err != nil {
		tb.Fatalf("go csmith seed=%d: %v", seed, err)
	}
	return out
}

// bodyMismatchReport is a capped go-cmp style report (first differing lines).
// Full-program cmp.Diff can be multi-MB and break fuzz worker IPC; still uses
// go-cmp for the window around the first divergence.
func bodyMismatchReport(seed uint64, upBody, goBody string) string {
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
	if first < 0 {
		return fmt.Sprintf("seed=%d program body mismatch (len up=%d go=%d, no line diff?)",
			seed, len(upBody), len(goBody))
	}
	// Window of ±5 lines around first divergence, go-cmp on that slice.
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
	return fmt.Sprintf("seed=%d program body mismatch at line %d (-upstream +go):\n%s",
		seed, first+1, diff)
}

func assertSeedBodyParity(tb testing.TB, seed uint64) {
	tb.Helper()
	upOut := upstreamGenerate(tb, seed)
	goOut := goGenerate(tb, seed)
	upBody, err := programBody(upOut)
	if err != nil {
		tb.Fatalf("seed=%d upstream body: %v", seed, err)
	}
	goBody, err := programBody(goOut)
	if err != nil {
		tb.Fatalf("seed=%d go body: %v", seed, err)
	}
	if goBody == upBody {
		return
	}
	tb.Fatal(bodyMismatchReport(seed, upBody, goBody))
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

// FuzzBodyParity is SPEC §3.5a level C via testing.F (quick / short seeds).
//
//	CSMITH_UPSTREAM=... go test ./test/bodyparity -run '^$' -fuzz=FuzzBodyParity -fuzztime=30s
//
// Hard limit: Go internal/fuzz worker panics "deadlocked!" if a single input
// takes >10s (worker.go AfterFunc(10*time.Second)). Full upstream+go generate
// often exceeds that → "hung or terminated unexpectedly: exit status 2" with
// crashers that MATCH alone. For substantial level-C time use
// TestBodyParityLevelC (sequential random, no 10s worker limit).
//
// Does **not** re-seed the level-B battery (that is TestBodyParityBattery only).
// Mutator starts from the zero seed; interesting/failing inputs accumulate under
// testdata/fuzz/. Commit crashers only as intentional regressions while C is open.
func FuzzBodyParity(f *testing.F) {
	f.Logf("upstream=%s", upstreamCsmith(f))
	// One trivial entry so continuous -fuzz has a root; avoids re-running the
	// full battery as baseline (duplicate of TestBodyParityBattery).
	f.Add(uint64(0))
	f.Fuzz(func(t *testing.T, seed uint64) {
		assertSeedBodyParity(t, seed)
	})
}

// TestBodyParityLevelC is SPEC §3.5a level C without the go-fuzz 10s worker cap:
// sequential random seeds, exact pre-stats body, until BODYPARITY_LEVELC duration
// elapses (default 10m when set to "1"/"true"; or a Go duration like "10m").
//
//	CSMITH_UPSTREAM=... BODYPARITY_LEVELC=10m go test ./test/bodyparity \
//	  -run TestBodyParityLevelC -count=1 -timeout 15m
//
// Skipped unless BODYPARITY_LEVELC is set (keeps default `go test` fast).
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
	t.Logf("upstream=%s levelC duration=%s", os.Getenv("CSMITH_UPSTREAM"), dur)
	deadline := time.Now().Add(dur)
	// Deterministic start; then walk a large stride so re-runs explore more.
	seed := uint64(0x9e3779b97f4a7c15) // golden ratio step
	n, start := 0, time.Now()
	for time.Now().Before(deadline) {
		n++
		// SplitMix64-style step for non-repeating seed stream
		seed += 0x9e3779b97f4a7c15
		z := seed
		z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
		z = (z ^ (z >> 27)) * 0x94d049bb133111eb
		z = z ^ (z >> 31)
		assertSeedBodyParity(t, z)
		if t.Failed() {
			t.Logf("levelC failed after n=%d elapsed=%s", n, time.Since(start).Round(time.Second))
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
	if strings.Contains(body, "Generator:") {
		t.Fatal("header should be excluded")
	}
}

func TestBodyDiffUsesCmp(t *testing.T) {
	want := "a\nb\nc"
	got := "a\nX\nc"
	diff := cmp.Diff(strings.Split(want, "\n"), strings.Split(got, "\n"))
	if diff == "" {
		t.Fatal("expected non-empty diff")
	}
	if !strings.Contains(diff, "b") || !strings.Contains(diff, "X") {
		t.Fatalf("unexpected diff:\n%s", diff)
	}
}
