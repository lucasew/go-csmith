package csmith

import "testing"

func TestMakeRandomDrawCounts(t *testing.T) {
	// Measure rand_depth cost of make_random for char vs short
	for _, st := range []ESimpleType{EChar, EShort, EInt} {
		o := Defaults()
		s := NewSession(o)
		r := NewRngSess(s, 2)
		s.Rng = r
		s.Probs = NewProbabilities(o)
		typ := GetSimpleTypeSess(s, st)
		d0 := r.RandDepthSess(s)
		c := MakeRandomSess(s, typ, o, s.Probs, r)
		d1 := r.RandDepthSess(s)
		t.Logf("st=%d depth+=%d val=%q", st, d1-d0, c.Value)
	}
	// Sequence: 500 char then 1 short from seed 2 — print first short value
	o := Defaults()
	o.CoverageTest = true
	s := NewSession(o)
	r := NewRngSess(s, 2)
	s.Rng, s.Probs = r, NewProbabilities(o)
	// mimic post-gen depth by running full gen first
	g := NewProgramGenerator(s)
	g.Initialize()
	g.GenerateAllTypes()
	g.GenerateFunctions()
	// After GenerateFunctions, CoverageTests already filled via ExtensionMgrGenerateValues
	if len(s.CoverageTests) < 501 {
		t.Fatalf("tests %d", len(s.CoverageTests))
	}
	t.Logf("a0[0]=%s a0[499]=%s a1[0]=%s a1[1]=%s",
		s.CoverageTests[0].Value, s.CoverageTests[499].Value,
		s.CoverageTests[500].Value, s.CoverageTests[501].Value)
}
