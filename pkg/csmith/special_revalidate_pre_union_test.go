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
// CheckReadVar/IsNonreadableFieldSess(testAmbientSession, same as C++ FactVec pre_facts).
func TestSpecialRevalidatePreUnionReadableField(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntTypeSess(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EShort), GetSimpleTypeSess(testAmbientSession, EUInt)}
	ut := MakeRandomUnionType(NewRngSess(testAmbientSession, 5), opts, probs, &env, "U0")
	if ut == nil || len(ut.Fields) < 2 {
		t.Skip("need union with ≥2 fields")
	}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) < 2 {
		t.Skip("fields")
	}
	f0, f1 := uv.FieldVars[0], uv.FieldVars[1]

	// Pre-statement lattice: last write field 0 → f0 readable.
	preUnion := []*FactUnion{MakeFactUnionSess(testAmbientSession, uv, 0)}
	// Post-gen lattice: nested body last-wrote field 1 → f0 nonreadable.
	postGenUnion := []*FactUnion{MakeFactUnionSess(testAmbientSession, uv, 1)}

	if IsNonreadableFieldSess(testAmbientSession, f0, preUnion) {
		t.Fatal("pre-union last=f0: f0 must be readable (special revalidate base)")
	}
	if !IsNonreadableFieldSess(testAmbientSession, f0, postGenUnion) {
		t.Fatal("post-gen last=f1: f0 must be nonreadable (must not be revalidate base)")
	}
	if IsNonreadableFieldSess(testAmbientSession, f1, postGenUnion) {
		t.Fatal("post-gen last=f1: f1 readable")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("complete paths must not sticky: %v", GetErrorSess(testAmbientSession))
	}

	// Deep clone of pre must not alias post-gen mutations (visit Join/SetBottom).
	work := CloneUnionFactSliceDeepSess(testAmbientSession, preUnion)
	if !UnionFactsComplete(work) || HasErrorSess(testAmbientSession) {
		t.Fatalf("deep clone pre incomplete err=%v", GetErrorSess(testAmbientSession))
	}
	// Mutate work in place like visit.
	if len(work) > 0 && work[0] != nil {
		work[0].LastWrittenFID = 1
	}
	// preUnion snapshot must stay last=0.
	if preUnion[0].LastWrittenFID != 0 {
		t.Fatalf("shallow alias: preUnion last=%d want 0", preUnion[0].LastWrittenFID)
	}
	if IsNonreadableFieldSess(testAmbientSession, f0, preUnion) {
		t.Fatal("preUnion must remain last=f0 after deep-clone work mutation")
	}
	if !IsNonreadableFieldSess(testAmbientSession, f0, work) {
		t.Fatal("mutated work last=f1 must block f0")
	}
}
