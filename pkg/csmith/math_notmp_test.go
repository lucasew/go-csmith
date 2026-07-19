package csmith

import (
	"strings"
	"testing"
)

func TestMathNoTmpBinaryOutput(t *testing.T) {
	opts := Defaults()
	opts.SafeMath = true
	opts.MathNoTmp = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	var fi *Invocation
	for seed := uint64(1); seed < 80; seed++ {
		// reset tmp vars
		blk.TmpVars = nil
		fi = MakeRandomBinaryInvocation(NewRng(seed), opts, probs, vs, tables, &cg, GetIntType())
		if fi != nil && fi.Safe != nil && fi.MathNoTmp && fi.Tmp1 != "" && SafeOpsBinary(fi.Binary) {
			break
		}
		fi = nil
	}
	if fi == nil {
		t.Skip("no math_notmp safe binary in sample")
	}
	out := fi.Output()
	if !strings.Contains(out, fi.Tmp1) || !strings.Contains(out, "safe_") {
		t.Fatal(out)
	}
	// block should list tmp decls when output
	body := blk.Output(0)
	if !strings.Contains(body, fi.Tmp1) {
		// TmpVars only emitted if Output is on block that has them
		if len(blk.TmpVars) == 0 {
			t.Fatal("no tmp vars registered")
		}
	}
}

func TestTmpVarsEmitSorted(t *testing.T) {
	b := &Block{TmpVars: map[string]ESimpleType{
		"t_3": EInt,
		"t_1": EInt,
		"t_2": EShort,
	}}
	out := b.Output(0)
	i1 := strings.Index(out, "t_1")
	i2 := strings.Index(out, "t_2")
	i3 := strings.Index(out, "t_3")
	if i1 < 0 || i2 < 0 || i3 < 0 || !(i1 < i2 && i2 < i3) {
		t.Fatal(out)
	}
	// empty tmp name — no invent "int  = 0;"
	b2 := &Block{TmpVars: map[string]ESimpleType{"": EInt, "t_ok": EInt}}
	out2 := b2.Output(0)
	if strings.Contains(out2, "int  = 0") || strings.Contains(out2, "  = 0") {
		t.Fatal("empty tmp name must not invent decl", out2)
	}
	if !strings.Contains(out2, "t_ok") {
		t.Fatal(out2)
	}
}

func TestNoteReadTracksGlobal(t *testing.T) {
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect())
	g := CreateVariableQfer("g_1", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	cg.NoteRead(g)
	if !f.FEffect.IsRead(g) {
		t.Fatal("read")
	}
	out := f.FEffect.CommentOutput()
	if !strings.Contains(out, "reads :") || !strings.Contains(out, "g_1") {
		t.Fatal(out)
	}
}
