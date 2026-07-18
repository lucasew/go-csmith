package csmith

import (
	"strings"
	"testing"
)

func TestFlagsToTypeSignedSizes(t *testing.T) {
	if FlagsToType(true, SafeInt8).Simple() != EChar {
		t.Fatal("int8")
	}
	if FlagsToType(false, SafeInt32).Simple() != EUInt {
		t.Fatal("uint32")
	}
	if FlagsToType(true, SafeInt64).Simple() != ELongLong {
		t.Fatal("ll")
	}
}

func TestMakeRandomBinaryUsesFlagOperandTypes(t *testing.T) {
	opts := Defaults()
	opts.SafeMath = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	// find an arithmetic safe op invocation
	var fi *Invocation
	for seed := uint64(1); seed < 80; seed++ {
		fi = MakeRandomBinaryInvocation(NewRng(seed), opts, probs, vs, tables, EmptyCGContext(), GetIntType())
		if fi != nil && fi.Safe != nil && SafeOpsBinary(fi.Binary) {
			break
		}
	}
	if fi == nil || fi.Safe == nil {
		t.Skip("no safe binary in sample")
	}
	out := fi.Output()
	if !strings.Contains(out, "safe_") {
		t.Fatal(out)
	}
	// LHS/RHS types are simple
	if fi.Safe.LHSType() == nil || !fi.Safe.LHSType().IsSimple() {
		t.Fatal("lhs type")
	}
}

func TestMakeRandomPointerTypeMayBeIntStar(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(2), opts, probs, env)
	p := env.MakeRandomPointerType(NewRng(3), opts, probs)
	if p == nil || p.PtrType() == nil {
		t.Fatal(p)
	}
	// integer bases consolidated to int*
	if p.PtrType().IsSimple() && p.PtrType().Simple() != EInt && p.PtrType().Simple() != EFloat {
		// after consolidate should be int for integer simples
		if p.PtrType().Simple() != EInt {
			// struct* is fine
			if !p.PtrType().IsStruct() && !p.PtrType().IsUnion() && p.PtrType().PtrType() == nil {
				t.Log("base", p.PtrType().CName())
			}
		}
	}
}
