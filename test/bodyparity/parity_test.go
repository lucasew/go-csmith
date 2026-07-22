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
	diff := cmp.Diff(strings.Split(upBody, "\n"), strings.Split(goBody, "\n"))
	tb.Fatalf("seed=%d program body mismatch (-upstream +go):\n%s", seed, diff)
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

// FuzzBodyParity is SPEC §3.5a level C via testing.F.
//
//	CSMITH_UPSTREAM=... go test ./test/bodyparity -run '^$' -fuzz=FuzzBodyParity -fuzztime=30s
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
