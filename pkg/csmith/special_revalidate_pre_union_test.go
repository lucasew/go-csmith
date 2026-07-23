package csmith

import "testing"

// TestSpecialRevalidatePreUnionReadableField documents seed-10054:
// Statement.cpp:1006–1012 FactVec outputs = pre_facts is the full FactVec
// (ePointTo + eUnionWrite). Soft invent revalidated from pre point-to only while
// FM.UnionFacts stayed post-gen: gen of nested func_30 last-wrote g_582.f4 so
// IsNonreadableField rejected g_582.f0 during special revalidate arg visit,
// aborting validate and leaving gen-time map_stm_effect IV extras.
//
// Contract: pre eUnionWrite lattice must be the special-revalidate base for
// CheckReadVar/IsNonreadableField (same as C++ FactVec pre_facts).
func TestSpecialRevalidatePreUnionReadableField(t *testing.T) {
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(5), opts, probs, &env, "U0")
	if ut == nil || len(ut.Fields) < 2 {
		t.Skip("need union with ≥2 fields")
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) < 2 {
		t.Skip("fields")
	}
	f0, f1 := uv.FieldVars[0], uv.FieldVars[1]

	// Pre-statement lattice: last write field 0 → f0 readable.
	preUnion := []*FactUnion{MakeFactUnion(uv, 0)}
	// Post-gen lattice: nested body last-wrote field 1 → f0 nonreadable.
	postGenUnion := []*FactUnion{MakeFactUnion(uv, 1)}

	if IsNonreadableField(f0, preUnion) {
		t.Fatal("pre-union last=f0: f0 must be readable (special revalidate base)")
	}
	if !IsNonreadableField(f0, postGenUnion) {
		t.Fatal("post-gen last=f1: f0 must be nonreadable (must not be revalidate base)")
	}
	if IsNonreadableField(f1, postGenUnion) {
		t.Fatal("post-gen last=f1: f1 readable")
	}
	if HasError() {
		t.Fatalf("complete paths must not sticky: %v", GetError())
	}

	// Deep clone of pre must not alias post-gen mutations (visit Join/SetBottom).
	work := CloneUnionFactSliceDeep(preUnion)
	if !UnionFactsComplete(work) || HasError() {
		t.Fatalf("deep clone pre incomplete err=%v", GetError())
	}
	// Mutate work in place like visit.
	if len(work) > 0 && work[0] != nil {
		work[0].LastWrittenFID = 1
	}
	// preUnion snapshot must stay last=0.
	if preUnion[0].LastWrittenFID != 0 {
		t.Fatalf("shallow alias: preUnion last=%d want 0", preUnion[0].LastWrittenFID)
	}
	if IsNonreadableField(f0, preUnion) {
		t.Fatal("preUnion must remain last=f0 after deep-clone work mutation")
	}
	if !IsNonreadableField(f0, work) {
		t.Fatal("mutated work last=f1 must block f0")
	}
}
