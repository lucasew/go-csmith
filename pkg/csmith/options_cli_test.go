package csmith

import (
	"reflect"
	"testing"
)

func TestOptionsRegistryCoversEveryStructField(t *testing.T) {
	rt := reflect.TypeOf(Options{})
	registered := make(map[string]bool)
	for _, n := range OptionsRegistryNames() {
		registered[n] = true
	}
	var missing []string
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		if !registered[f.Name] {
			missing = append(missing, f.Name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("Options fields not in optionsRegistry: %v", missing)
	}
	// Extra registry names that aren't on the struct
	var extra []string
	for n := range registered {
		if _, ok := rt.FieldByName(n); !ok {
			extra = append(extra, n)
		}
	}
	if len(extra) > 0 {
		t.Fatalf("registry names not on Options: %v", extra)
	}
}

func TestCLIArgsDefaultsSeedOnly(t *testing.T) {
	o := Defaults()
	o.Seed = 2
	args := o.CLIArgs()
	want := []string{"-s", "2"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("CLIArgs defaults: got %v want %v", args, want)
	}
}

func TestCLIArgsNoJumps(t *testing.T) {
	o := Defaults()
	o.Seed = 42
	o.Jumps = false
	args := o.CLIArgs()
	want := []string{"-s", "42", "--no-jumps"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("got %v want %v", args, want)
	}
}

func TestCLIArgsDoesNotEmitMaxParams(t *testing.T) {
	o := Defaults()
	o.Seed = 1
	o.MaxParams = 2
	for _, a := range o.CLIArgs() {
		if a == "--max-params" {
			t.Fatal("must not emit --max-params (golden rejects)")
		}
	}
	if gaps := o.UpstreamParityGaps(); len(gaps) == 0 || gaps[0] != "MaxParams" {
		// may include only MaxParams
		found := false
		for _, g := range gaps {
			if g == "MaxParams" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected MaxParams in gaps, got %v", gaps)
		}
	}
}

func TestCLIArgsDoesNotEmitDepthProtect(t *testing.T) {
	o := Defaults()
	o.DepthProtect = true
	for _, a := range o.CLIArgs() {
		if a == "--depth-protect" {
			t.Fatal("golden OMITs depth-protect")
		}
	}
}

func TestOptionsFromFuzzBlobSeedOnlyKeepsDefaults(t *testing.T) {
	b := make([]byte, 8)
	b[0] = 2 // seed 2, not version 2 — legacy path uses full 8 bytes as seed
	// seed=2 LE: 02 00 00 00 00 00 00 00
	b = []byte{2, 0, 0, 0, 0, 0, 0, 0}
	o := OptionsFromFuzzBlob(b)
	if o.Seed != 2 {
		t.Fatalf("seed: got %d", o.Seed)
	}
	d := Defaults()
	if o.Jumps != d.Jumps || o.MaxFuncs != d.MaxFuncs {
		t.Fatalf("legacy seed-only must keep Defaults: jumps=%v maxFuncs=%d", o.Jumps, o.MaxFuncs)
	}
}

func TestOptionsFromFuzzBlobV2RandomizesBools(t *testing.T) {
	o := Defaults()
	o.Seed = 7
	o.Jumps = false
	blob := FuzzBlobFromOptions(o)
	if blob[0] != fuzzBlobMagic || blob[1] != fuzzBlobVersion {
		t.Fatalf("magic/version %x %x", blob[0], blob[1])
	}
	got := OptionsFromFuzzBlob(blob)
	if got.Jumps {
		t.Fatal("expected jumps disabled after v2 round-trip")
	}
	if got.Seed != 7 {
		t.Fatalf("seed %d", got.Seed)
	}
}

func TestOptionsFromFuzzBlobV2RoundTripDefaults(t *testing.T) {
	o := Defaults()
	o.Seed = 99
	got := OptionsFromFuzzBlob(FuzzBlobFromOptions(o))
	if got.Seed != 99 {
		t.Fatalf("seed %d", got.Seed)
	}
	if got.Jumps != o.Jumps || got.SafeMath != o.SafeMath || got.MaxFuncs != o.MaxFuncs {
		t.Fatalf("round-trip mismatch: jumps=%v safe=%v maxFuncs=%d", got.Jumps, got.SafeMath, got.MaxFuncs)
	}
	// Ints outside the compact fuzz span (or equal to Defaults) must survive:
	// previously MaxExhaustiveDepth=-1 and CoverageTestSize=500 were clamped into
	// the span, so FuzzBlobFromOptions(Defaults()) emitted non-default CLI flags.
	if got.MaxExhaustiveDepth != o.MaxExhaustiveDepth {
		t.Fatalf("MaxExhaustiveDepth: got %d want %d", got.MaxExhaustiveDepth, o.MaxExhaustiveDepth)
	}
	if got.CoverageTestSize != o.CoverageTestSize {
		t.Fatalf("CoverageTestSize: got %d want %d", got.CoverageTestSize, o.CoverageTestSize)
	}
	// Drop-in Defaults blob should only need -s on the golden CLI.
	args := got.CLIArgs()
	if len(args) != 2 || args[0] != "-s" || args[1] != "99" {
		t.Fatalf("Defaults round-trip CLIArgs=%v want [-s 99]", args)
	}
}

func TestFuzzPlanesNonZero(t *testing.T) {
	nb, ni, ns := OptionsFieldCount()
	if nb < 80 {
		t.Fatalf("bool plane too small: %d", nb)
	}
	if ni < 20 {
		t.Fatalf("int plane too small: %d", ni)
	}
	if ns < 10 {
		t.Fatalf("str plane too small: %d", ns)
	}
	db, di, ds := DropInFieldCount()
	if db < 60 || di < 15 {
		t.Fatalf("drop-in planes too small: bool=%d int=%d str=%d", db, di, ds)
	}
	if db > nb || di > ni {
		t.Fatalf("drop-in larger than full registry")
	}
	t.Logf("full bool=%d int=%d str=%d; drop-in bool=%d int=%d str=%d", nb, ni, ns, db, di, ds)
}

func TestForDropInParityClearsLibraryOnly(t *testing.T) {
	o := Defaults()
	o.Seed = 9
	o.Jumps = false
	o.MaxParams = 2        // library-only
	o.WrapVolatiles = true // library-only
	o.MaxGlobals = 10      // go-only
	got := o.ForDropInParity()
	if got.Jumps {
		t.Fatal("CLI field Jumps should stay false")
	}
	if got.MaxParams != Defaults().MaxParams {
		t.Fatalf("MaxParams should reset to default, got %d", got.MaxParams)
	}
	if got.WrapVolatiles || got.MaxGlobals != 0 {
		t.Fatalf("library/go-only not cleared: wrap=%v maxG=%d", got.WrapVolatiles, got.MaxGlobals)
	}
	if gaps := got.UpstreamParityGaps(); len(gaps) != 0 {
		t.Fatalf("gaps after ForDropInParity: %v", gaps)
	}
}

func TestOptionsFromFuzzBlobDoesNotSetMaxParams(t *testing.T) {
	// Even a dense blob must leave library MaxParams at default.
	o := Defaults()
	o.Seed = 1
	o.Jumps = false
	o.MaxParams = 2
	blob := FuzzBlobFromOptions(o)
	got := OptionsFromFuzzBlob(blob)
	if got.MaxParams != Defaults().MaxParams {
		t.Fatalf("drop-in fuzz must not change MaxParams: %d", got.MaxParams)
	}
	if got.Jumps {
		t.Fatal("want jumps false")
	}
}

func TestCLIArgsTakeUnionFieldAddr(t *testing.T) {
	o := Defaults()
	o.TakeUnionFieldAddr = false
	args := o.CLIArgs()
	found := false
	for _, a := range args {
		if a == "--take-no-union-field-addr" {
			found = true
		}
		if a == "--no-take-union-field-addr" {
			t.Fatalf("wrong negative form in %v", args)
		}
	}
	if !found {
		t.Fatalf("missing --take-no-union-field-addr in %v", args)
	}
	o.TakeUnionFieldAddr = true
	// default true → no flag
	for _, a := range o.CLIArgs() {
		if a == "--take-union-field-addr" || a == "--take-no-union-field-addr" {
			t.Fatalf("default should omit flag, got %v", o.CLIArgs())
		}
	}
}
