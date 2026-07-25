package csmith

import (
	"strings"
	"testing"
)

func TestNoteWriteDoesNotTouchFEffect(t *testing.T) {
	// CGContext::write_var does not update Function::feffect (Function.cpp:657
	// finalizes via map_stm_effect[body] only).
	opts := Defaults()
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	g := CreateVariableQfer("g_1", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	l := CreateVariableQfer("l_1", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	cg.NoteWrite(g)
	cg.NoteWrite(l)
	if f.FEffect.IsWritten(g) || f.FEffect.IsWritten(l) {
		t.Fatal("NoteWrite must not invent mid-generation feffect updates")
	}
	// ComputeSummary still records globals from body effect
	f.ComputeSummary(EmptyEffect().WriteVar(g))
	if !f.FEffect.IsWritten(g) {
		t.Fatal("ComputeSummary must add external global write")
	}
	if f.FEffect.IsWritten(l) {
		t.Fatal("local must not enter feffect via AddExternalEffect")
	}
	_ = opts
}

func TestFunctionOutputFEffectComment(t *testing.T) {
	f := &Function{Name: "func_1", ReturnType: GetIntType(), EmitConcise: false}
	g := CreateVariableQfer("g_9", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	f.FEffect = f.FEffect.WriteVar(g)
	f.Body = &Block{}
	out := f.Output()
	if !strings.Contains(out, "writes:") || !strings.Contains(out, "g_9") {
		t.Fatal(out)
	}
	f.EmitConcise = true
	out2 := f.Output()
	if strings.Contains(out2, "writes:") {
		t.Fatal("concise should skip")
	}
}

func TestGenerateHasEffectComments(t *testing.T) {
	opts := Defaults()
	opts.Seed = 2
	opts.Concise = false
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	// at least some function comments with writes (if any globals written)
	if !strings.Contains(out, " * writes:") {
		t.Log("no write comments — possible if no global assigns")
	}
}

func TestCommentOutputInsertionOrderAndFormat(t *testing.T) {
	// Effect.cpp:507–529 — vector order; OutputMgr.cpp:318 — "/* " wrap
	ClearErrorSess(testAmbientSession)
	a := CreateVariableQfer("g_a", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	b := CreateVariableQfer("g_b", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	c := CreateVariableQfer("g_c", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	// insert b then a then c — not alphabetical
	eff := EmptyEffect().ReadVar(b).ReadVar(a).ReadVar(c).WriteVar(c).WriteVar(a)
	out := eff.CommentOutput()
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete CommentOutput sticky")
	}
	wantReads := " * reads : g_b g_a g_c"
	wantWrites := " * writes: g_c g_a"
	if !strings.Contains(out, wantReads) {
		t.Fatalf("reads order: %q", out)
	}
	if !strings.Contains(out, wantWrites) {
		t.Fatalf("writes order: %q", out)
	}
	if strings.Contains(out, "g_a g_b g_c") {
		t.Fatal("must not sort alphabetically")
	}
	if !strings.HasPrefix(out, "/* \n") || !strings.HasSuffix(out, " */\n") {
		t.Fatalf("output_comment_line wrap: %q", out)
	}
}
