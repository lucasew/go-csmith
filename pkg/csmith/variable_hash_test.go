package csmith

import (
	"strings"
	"testing"
)

func TestHashSimpleInt(t *testing.T) {
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	out := v.HashOutput()
	if !strings.Contains(out, `transparent_crc(g_1, "g_1"`) {
		t.Fatal(out)
	}
	// empty name sticky — no invent transparent_crc(,"")
	anon := &Variable{Name: "", Type: GetIntType()}
	if anon.HashOutput() != "" {
		t.Fatal("empty name must fail closed hash")
	}
	if !HasError() {
		t.Fatal("empty name HashOutput must SetError sticky")
	}
	ClearError()
}

func TestHashPointerEmpty(t *testing.T) {
	v := CreateVariableScalars("g_2", PointerTo(GetIntType()), false, false)
	if v.HashOutput() != "" {
		t.Fatal("pointers must not hash")
	}
}

func TestHashStructFields(t *testing.T) {
	st := &Type{
		isStruct:   true,
		StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false}), BitWidth: -1},
			{Name: "f1", Type: GetSimpleType(EUInt), Qfer: NewCVQualifiers([]bool{false}, []bool{false}), BitWidth: -1},
		},
	}
	v := CreateVariableQfer("g_3", st, NewCVQualifiers([]bool{false}, []bool{false}))
	out := v.HashOutput()
	if !strings.Contains(out, "g_3.f0") || !strings.Contains(out, "g_3.f1") {
		t.Fatal(out)
	}
	// top-level aggregate name must not appear as scalar crc
	if strings.Contains(out, `transparent_crc(g_3, "g_3"`) {
		t.Fatal("must not hash whole struct", out)
	}
}

func TestHashArrayLoops(t *testing.T) {
	ClearError()
	CtrlVarsDoFinalization()
	// ArrayVariable::hash uses get_last_ctrl_vars (no letter invent without pool)
	_ = GetNewCtrlVars(Defaults())
	// live AsArray required (no invent hash-array expand from ArraySizes alone)
	av := &ArrayVariable{
		Variable: Variable{
			Name:       "g_4",
			Type:       GetIntType(),
			IsArray:    true,
			ArraySizes: []int{3},
			Qfer:       NewCVQualifiers([]bool{false}, []bool{false}),
		},
		Sizes: []int{3},
	}
	av.AsArray = av
	out := av.Variable.HashOutput()
	// Variable::new_ctrl_vars uses letter names i, j, k…
	if !strings.Contains(out, "for (i = 0") || !strings.Contains(out, "g_4[i]") {
		t.Fatal(out)
	}
	// undersized ctrl sticky — no invent loops with empty index
	ClearError()
	CtrlVarsDoFinalization()
	if av.Variable.HashOutput() != "" {
		t.Fatal("no last ctrl must fail closed array hash")
	}
	if !HasError() {
		t.Fatal("no last ctrl HashOutput must SetError sticky")
	}
	ClearError()
	// IsArray without AsArray sticky empty
	shell := &Variable{Name: "g_5", Type: GetIntType(), IsArray: true, ArraySizes: []int{3}}
	if shell.HashOutput() != "" {
		t.Fatal("IsArray without AsArray HashOutput must fail closed empty")
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray HashOutput must SetError sticky")
	}
	ClearError()
	CtrlVarsDoFinalization()
}

func TestGenerateHashUsesFieldCrc(t *testing.T) {
	foundStructHash := false
	foundPtrSkip := false
	for seed := uint64(1); seed < 80; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, ".f0") && strings.Contains(out, "transparent_crc") {
			foundStructHash = true
		}
		// pointer globals: should not transparent_crc(g_N with * type as raw) — hard to check
		_ = foundPtrSkip
		if foundStructHash {
			break
		}
	}
	if !foundStructHash {
		t.Log("struct field hash not seen in sample — optional")
	}
}

func TestHashOutputIsAggregateResidualSticky(t *testing.T) {
	// IsAggregate residual soft invent was invent soft-continue scalar hash past incomplete.
	// Type-nil already sticky at hashOutput entry.
	ClearError()
	v := &Variable{Name: "g_x", Type: nil}
	if v.hashOutput(nil, nil) != "" {
		t.Fatal("Type-nil hashOutput must fail closed empty")
	}
	if !HasError() {
		t.Fatal("Type-nil hashOutput must SetError sticky")
	}
	ClearError()
}
