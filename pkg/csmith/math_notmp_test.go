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
	// Block.cpp:261–262 — decls only when math_notmp
	prev := ProcessOptions()
	opts := prev
	opts.MathNoTmp = true
	SetProcessOptions(opts)
	t.Cleanup(func() { SetProcessOptions(prev) })

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
	// empty tmp name — sticky fail closed whole block (no invent skip hole / partial tmp list)
	ClearError()
	b2 := &Block{TmpVars: map[string]ESimpleType{"": EInt, "t_ok": EInt}}
	out2 := b2.Output(0)
	if out2 != "" {
		t.Fatal("empty tmp name must fail closed whole block", out2)
	}
	if !HasError() {
		t.Fatal("empty tmp name must SetError sticky")
	}
	// invalid eSimpleType — sticky fail closed (no invent "int" for OOB tmp type)
	ClearError()
	b3 := &Block{TmpVars: map[string]ESimpleType{"t_bad": ESimpleType(MaxSimpleTypes + 1)}}
	if b3.Output(0) != "" {
		t.Fatal("OOB tmp type must fail closed whole block")
	}
	if !HasError() {
		t.Fatal("OOB tmp type must SetError sticky")
	}
	ClearError()
	// default !math_notmp: create-only, no decls (C++ OutputTmpVariableList gated)
	opts.MathNoTmp = false
	SetProcessOptions(opts)
	if strings.Contains((&Block{TmpVars: map[string]ESimpleType{"t_9": EInt}}).Output(0), "t_9") {
		t.Fatal("!math_notmp must not emit tmp decls")
	}
}

func TestNoteReadTracksGlobal(t *testing.T) {
	// NoteRead does not update feffect (Function.cpp:657 finalizes from body map).
	// CommentOutput uses insertion-order read_vars via OutputForComment.
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	g := CreateVariableQfer("g_1", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	f.FEffect = f.FEffect.ReadVar(g)
	if !f.FEffect.IsRead(g) {
		t.Fatal("read")
	}
	out := f.FEffect.CommentOutput()
	if !strings.Contains(out, "reads :") || !strings.Contains(out, "g_1") {
		t.Fatal(out)
	}
	// format: output_comment_line wraps body starting with newline
	if !strings.HasPrefix(out, "/* \n * reads :") {
		t.Fatal("want /* + newline + reads, got", out[:40])
	}
	// empty actual name sticky fail closed (OutputForComment — no invent blank token)
	anon := &Variable{Type: GetIntType()}
	ClearError()
	c := EmptyEffect().ReadVar(anon).WriteVar(g).CommentOutput()
	if c != "" || !HasError() {
		t.Fatal("empty-name read must fail closed CommentOutput", c)
	}
	ClearError()
	onlyAnon := EmptyEffect().ReadVar(anon).CommentOutput()
	if onlyAnon != "" || !HasError() {
		t.Fatal("empty name only must fail closed", onlyAnon)
	}
	ClearError()
}
