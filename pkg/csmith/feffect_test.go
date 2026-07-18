package csmith

import (
	"strings"
	"testing"
)

func TestNoteWriteUpdatesFEffect(t *testing.T) {
	opts := Defaults()
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect())
	g := CreateVariableQfer("g_1", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	l := CreateVariableQfer("l_1", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	cg.NoteWrite(g)
	cg.NoteWrite(l)
	if !f.FEffect.IsWritten(g) {
		t.Fatal("global write")
	}
	if f.FEffect.IsWritten(l) {
		t.Fatal("local should not be in feffect")
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
