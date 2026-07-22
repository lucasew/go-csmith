// Upstream body parity tests (SPEC §3.5 / §3.5a).
//
// Compares generated C program bodies from this library vs an upstream csmith
// binary. Not a unit contract test — integration thermometer + level B/C gate.
//
// Skipped unless CSMITH_UPSTREAM points at an upstream binary (no path search).
// Unset → WARNING + Skip so plain `go test ./pkg/csmith` stays offline-friendly.
//
//	# Level B battery (testing.T)
//	CSMITH_UPSTREAM=/path/to/csmith go test ./pkg/csmith -run UpstreamBodyParityBattery -count=1
//
//	# Level C fuzzy (testing.F; opt-in via -fuzz; N seeds via -fuzztime=Nx)
//	CSMITH_UPSTREAM=/path/to/csmith go test ./pkg/csmith -run '^$' \
//	  -fuzz=FuzzUpstreamBodyParityFuzzy -fuzztime=16x -count=1 -timeout 30m
package csmith

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// Frozen battery from SPEC §3.5a (level B). Do not shrink in this list.
var upstreamBodyParityBattery = []uint64{
	0, 1, 2, 3, 4, 5, 7, 10, 42, 100, 123, 999,
}

var (
	reBodyStartStatic = regexp.MustCompile(`(?m)^static long __undefined;`)
	reBodyStartStruct = regexp.MustCompile(`(?m)^/\* --- Struct/Union Declarations --- \*/`)
	reBodyEndStats    = regexp.MustCompile(`(?m)^/\*{10,} statistics`)
	reBodyEndXXX      = regexp.MustCompile(`(?m)^XXX number of`)
)

// ProgramBody extracts the SPEC §3.5 program body: from the first program
// section after the banner/includes through (not including) the statistics tail.
// Header Generator/Git lines are outside this slice by construction.
func ProgramBody(src string) (string, error) {
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

// findUpstreamCsmith returns the path from CSMITH_UPSTREAM only.
// Unset → log a warning and return "" (callers Skip). No auto-discovery of
// .build or PATH — that looked like "upstream in radar" when the env was empty.
func findUpstreamCsmith(tb testing.TB) string {
	tb.Helper()
	env := strings.TrimSpace(os.Getenv("CSMITH_UPSTREAM"))
	if env == "" {
		tb.Log("WARNING: CSMITH_UPSTREAM is not set; skipping upstream body parity")
		return ""
	}
	if st, err := os.Stat(env); err != nil || st.IsDir() {
		tb.Fatalf("CSMITH_UPSTREAM=%q not a file", env)
	}
	abs, err := filepath.Abs(env)
	if err != nil {
		return env
	}
	return abs
}

func genTimeout(tb testing.TB) time.Duration {
	tb.Helper()
	if s := strings.TrimSpace(os.Getenv("CSMITH_FUZZY_TIMEOUT_SEC")); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			tb.Fatalf("CSMITH_FUZZY_TIMEOUT_SEC=%q", s)
		}
		return time.Duration(n) * time.Second
	}
	return 120 * time.Second
}

func runUpstream(tb testing.TB, bin string, seed uint64, timeout time.Duration) (string, error) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-s", strconv.FormatUint(seed, 10))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("upstream timeout after %s", timeout)
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("upstream: %s", msg)
	}
	return stdout.String(), nil
}

func runGo(tb testing.TB, seed uint64) (string, error) {
	tb.Helper()
	opts := Defaults()
	opts.Seed = seed
	return Generate(opts)
}

// programBodies returns (goBody, upstreamBody) for defaults + seed.
func programBodies(tb testing.TB, upBin string, seed uint64, timeout time.Duration) (goBody, upBody string, err error) {
	tb.Helper()
	upOut, err := runUpstream(tb, upBin, seed, timeout)
	if err != nil {
		return "", "", fmt.Errorf("upstream generate: %w", err)
	}
	goOut, err := runGo(tb, seed)
	if err != nil {
		return "", "", fmt.Errorf("go generate: %w", err)
	}
	upBody, err = ProgramBody(upOut)
	if err != nil {
		return "", "", fmt.Errorf("upstream body: %w", err)
	}
	goBody, err = ProgramBody(goOut)
	if err != nil {
		return "", "", fmt.Errorf("go body: %w", err)
	}
	return goBody, upBody, nil
}

// requireBodyMatch fails with a go-cmp line-oriented diff (upstream want, go got).
func requireBodyMatch(tb testing.TB, seed uint64, goBody, upBody string) {
	tb.Helper()
	if goBody == upBody {
		return
	}
	// Diff line slices so multi-KB C bodies stay readable (not one giant string atom).
	want := strings.Split(upBody, "\n")
	got := strings.Split(goBody, "\n")
	diff := cmp.Diff(want, got)
	tb.Fatalf("seed=%d program body mismatch (-upstream +go):\n%s", seed, diff)
}

func parseSeedList(s string) ([]uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]uint64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bad seed %q: %w", p, err)
		}
		out = append(out, n)
	}
	return out, nil
}

// TestUpstreamBodyParityBattery is SPEC §3.5a level B: frozen battery, exact body.
// Requires CSMITH_UPSTREAM or a discoverable upstream binary.
func TestUpstreamBodyParityBattery(t *testing.T) {
	if testing.Short() {
		t.Skip("short: skip upstream body parity")
	}
	up := findUpstreamCsmith(t)
	if up == "" {
		t.Skip("set CSMITH_UPSTREAM to the golden csmith binary")
	}
	t.Logf("upstream=%s", up)
	timeout := genTimeout(t)

	for _, seed := range upstreamBodyParityBattery {
		seed := seed
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			goBody, upBody, err := programBodies(t, up, seed, timeout)
			if err != nil {
				t.Fatal(err)
			}
			requireBodyMatch(t, seed, goBody, upBody)
		})
	}
}

// FuzzUpstreamBodyParityFuzzy is SPEC §3.5a level C: mutated seeds, exact program
// body, via testing.F.
//
// Only runs when an upstream binary is available; otherwise Skip. Continuous
// mutation is opt-in via `go test -fuzz=...` (not run by plain `go test` beyond
// the seed corpus). Corpus defaults to the level-B battery (known MATCH); extra
// seeds via CSMITH_FUZZY_SEEDS. Failures under -fuzz are written under
// testdata/fuzz/ — only commit those you intend as regression work items while
// level C is open.
//
//	go test ./pkg/csmith -run '^$' -fuzz=FuzzUpstreamBodyParityFuzzy -fuzztime=16x
//
// Env:
//
//	CSMITH_UPSTREAM          path to upstream binary (warn if unset)
//	CSMITH_FUZZY_SEEDS       comma list added to seed corpus
//	CSMITH_FUZZY_TIMEOUT_SEC per-side gen timeout seconds (default 120)
func FuzzUpstreamBodyParityFuzzy(f *testing.F) {
	if testing.Short() {
		f.Skip("short: skip upstream body fuzzy parity")
	}
	up := findUpstreamCsmith(f)
	if up == "" {
		f.Skip("set CSMITH_UPSTREAM to the golden csmith binary")
	}
	f.Logf("upstream=%s", up)
	timeout := genTimeout(f)

	for _, seed := range upstreamBodyParityBattery {
		f.Add(seed)
	}
	extra, err := parseSeedList(os.Getenv("CSMITH_FUZZY_SEEDS"))
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range extra {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, seed uint64) {
		goBody, upBody, err := programBodies(t, up, seed, timeout)
		if err != nil {
			t.Fatal(err)
		}
		requireBodyMatch(t, seed, goBody, upBody)
	})
}

// TestProgramBodyExtractSmoke locks body boundaries used by parity gates.
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
	body, err := ProgramBody(sample)
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

// TestBodyDiffUsesCmp is a tiny unit lock that mismatch reporting goes through go-cmp.
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
