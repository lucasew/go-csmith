package bodyparity_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"csmith/pkg/csmith"
)

func TestBodyParityPairCamp(t *testing.T) {
	raw := os.Getenv("BODYPARITY_PAIRCAMP")
	if raw == "" {
		t.Skip("set BODYPARITY_PAIRCAMP=2m")
	}
	dur, err := time.ParseDuration(raw)
	if err != nil {
		t.Fatal(err)
	}
	_ = upstreamCsmith(t)
	type mut struct {
		name string
		fn   func(*csmith.Options)
	}
	muts := []mut{
		{"crest", func(o *csmith.Options) { o.Crest = true }},
		{"klee", func(o *csmith.Options) { o.Klee = true }},
		{"access_once", func(o *csmith.Options) { o.AccessOnce = true }},
		{"check_global", func(o *csmith.Options) { o.BlindCheckGlobal = true }},
		{"mark_mutable", func(o *csmith.Options) { o.MarkMutableConst = true }},
		{"paranoid", func(o *csmith.Options) { o.Paranoid = true }},
		{"float", func(o *csmith.Options) { o.EnableFloat = true }},
		{"int128", func(o *csmith.Options) { o.Int128 = true }},
		{"uint128", func(o *csmith.Options) { o.UInt128 = true }},
		{"no_pointers", func(o *csmith.Options) { o.Pointers = false }},
		{"no_arrays", func(o *csmith.Options) { o.Arrays = false }},
		{"no_structs", func(o *csmith.Options) { o.Structs = false }},
		{"no_globals", func(o *csmith.Options) { o.GlobalVariables = false }},
		{"no_addr_locals", func(o *csmith.Options) { o.AddrTakenOfLocals = false }},
		{"type_attrs", func(o *csmith.Options) { o.TypeAttributes = true }},
		{"func_attrs", func(o *csmith.Options) { o.FunctionAttributes = true }},
		{"var_attrs", func(o *csmith.Options) { o.VariableAttributes = true }},
		{"coverage", func(o *csmith.Options) { o.CoverageTest = true }},
		{"identify_wrappers", func(o *csmith.Options) { o.IdentifyWrappers = true }},
		{"step_hash", func(o *csmith.Options) { o.StepHashByStmt = true }},
		{"binary_const", func(o *csmith.Options) { o.BinaryConstant = true }},
		{"compact", func(o *csmith.Options) { o.CompactOutput = true }},
		{"prefix_name", func(o *csmith.Options) { o.PrefixName = true }},
		{"no_safe_math", func(o *csmith.Options) { o.SafeMath = false }},
		{"expand_struct", func(o *csmith.Options) { o.ExpandStruct = true }},
		{"fixed_struct", func(o *csmith.Options) { o.FixedStructFields = true }},
		{"lang_cpp", func(o *csmith.Options) { o.LangCPP = true }},
		{"fast_exec", func(o *csmith.Options) { o.FastExecution = true }},
		{"strict_float", func(o *csmith.Options) { o.StrictFloat = true }},
		{"compatible", func(o *csmith.Options) { o.CompatibleCheck = true }},
	}
	seeds := []uint64{0, 1, 2, 3, 4, 5, 7, 10, 42, 100, 123, 999, 12345, 99999, 145079}
	deadline := time.Now().Add(dur)
	n := 0
	for si, seed := range seeds {
		for i := 0; i < len(muts) && time.Now().Before(deadline); i++ {
			for j := i; j < len(muts) && time.Now().Before(deadline); j++ {
				if i != j && (si+i+j)%3 != 0 {
					continue
				}
				n++
				mi, mj := muts[i], muts[j]
				name := fmt.Sprintf("s%d_%s", seed, mi.name)
				if i != j {
					name = fmt.Sprintf("s%d_%s+%s", seed, mi.name, mj.name)
				}
				ok := t.Run(name, func(t *testing.T) {
					o := csmith.Defaults()
					o.Seed = seed
					mi.fn(&o)
					if i != j {
						mj.fn(&o)
					}
					assertOptsBodyParity(t, o)
				})
				if !ok && t.Failed() {
					return
				}
			}
		}
	}
	t.Logf("paircamp CLEAN n=%d", n)
}
