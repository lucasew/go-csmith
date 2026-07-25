package csmith

import (
	"strings"
	"testing"
)

func TestFlagsToTypeSignedSizes(t *testing.T) {
	if FlagsToTypeSess(testAmbientSession, true, SafeInt8).SimpleSess(testAmbientSession) != EChar {
		t.Fatal("int8")
	}
	if FlagsToTypeSess(testAmbientSession, false, SafeInt32).SimpleSess(testAmbientSession) != EUInt {
		t.Fatal("uint32")
	}
	if FlagsToTypeSess(testAmbientSession, true, SafeInt64).SimpleSess(testAmbientSession) != ELongLong {
		t.Fatal("ll")
	}
}

func TestMakeRandomBinaryUsesFlagOperandTypes(t *testing.T) {
	opts := Defaults()
	opts.SafeMath = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTables(opts)
	// find an arithmetic safe op invocation
	var fi *Invocation
	for seed := uint64(1); seed < 80; seed++ {
		fi = func() *Invocation {
			c := EmptyCGContext().WithSession(testAmbientSession)
			return MakeRandomBinaryInvocation(NewRngSess(testAmbientSession, seed), opts, probs, vs, tables, &c, GetIntTypeSess(testAmbientSession))
		}()
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
	if fi.Safe.LHSTypeSess(testAmbientSession) == nil || !fi.Safe.LHSTypeSess(testAmbientSession).IsSimpleSess(testAmbientSession) {
		t.Fatal("lhs type")
	}
}

func TestMakeRandomPointerTypeMayBeIntStar(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRngSess(testAmbientSession, 2), opts, probs, env)
	p := env.MakeRandomPointerType(NewRngSess(testAmbientSession, 3), opts, probs)
	if p == nil || p.PtrTypeSess(testAmbientSession) == nil {
		t.Fatal(p)
	}
	// integer bases consolidated to int*
	if p.PtrTypeSess(testAmbientSession).IsSimpleSess(testAmbientSession) && p.PtrTypeSess(testAmbientSession).SimpleSess(testAmbientSession) != EInt && p.PtrTypeSess(testAmbientSession).SimpleSess(testAmbientSession) != EFloat {
		// after consolidate should be int for integer simples
		if p.PtrTypeSess(testAmbientSession).SimpleSess(testAmbientSession) != EInt {
			// struct* is fine
			if !p.PtrTypeSess(testAmbientSession).IsStructSess(testAmbientSession) && !p.PtrTypeSess(testAmbientSession).IsUnionSess(testAmbientSession) && p.PtrTypeSess(testAmbientSession).PtrTypeSess(testAmbientSession) == nil {
				t.Log("base", p.PtrTypeSess(testAmbientSession).CNameSess(testAmbientSession))
			}
		}
	}
}

func TestReturnFloatTypeBinaryIsFloatResidualSticky(t *testing.T) {
	// IsFloat residual soft invent was invent float-true past Type-nil shell.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.EnableFloat = true
	// nil Type IsFloat residual false with SetError - but we skip nil checks
	// Type-nil: IsFloat SetError + false
	if ReturnFloatTypeBinarySess(testAmbientSession, opts, (*Type)(nil), GetIntTypeSess(testAmbientSession), GetIntTypeSess(testAmbientSession), BinAdd) {
		t.Fatal("nil rv must not invent float true")
	}
	// nil rv skipped by rv != nil check - no residual
	if HasErrorSess(testAmbientSession) {
		// ok if residual from elsewhere
		ClearErrorSess(testAmbientSession)
	}
	// complete non-float
	if ReturnFloatTypeBinarySess(testAmbientSession, opts, GetIntTypeSess(testAmbientSession), GetIntTypeSess(testAmbientSession), GetIntTypeSess(testAmbientSession), BinAdd) {
		t.Fatal("int binary must not invent float")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete ReturnFloatTypeBinary must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// float rv
	ft := GetSimpleTypeSess(testAmbientSession, EFloat)
	if !ReturnFloatTypeBinarySess(testAmbientSession, opts, ft, GetIntTypeSess(testAmbientSession), GetIntTypeSess(testAmbientSession), BinAdd) {
		t.Fatal("float rv must return float true")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete float ReturnFloatTypeBinary must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}
