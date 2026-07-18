package csmith

import (
	"strings"
	"testing"
)

func TestOutputValueDumpSimple(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	out := v.OutputValueDump("checksum ", 1, nil)
	// int may be %d or %lld depending on platform SizeInBytes
	if !strings.Contains(out, "checksum g_1 = %") || !strings.Contains(out, ", g_1);") {
		t.Fatal(out)
	}
}

func TestOutputValueDumpStructFields(t *testing.T) {
	st := &Type{
		isStruct: true, StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetSimpleType(EUInt), BitWidth: -1},
		},
	}
	v := CreateVariableQfer("g_s", st, NewCVQualifiers([]bool{false}, []bool{false}))
	out := v.OutputValueDump("checksum ", 1, nil)
	if !strings.Contains(out, "g_s.f0") || !strings.Contains(out, "g_s.f1") {
		t.Fatal(out)
	}
	// unsigned int may be %u or %llu depending on SizeInBytes
	if !strings.Contains(out, "%u") && !strings.Contains(out, "%llu") {
		t.Fatal("uint directive", out)
	}
}

func TestOutputValueDumpUnionReadable(t *testing.T) {
	ut := &Type{
		isUnion: true, StructName: "U0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetIntType(), BitWidth: -1},
		},
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	// no facts → nothing readable
	if s := uv.OutputValueDump("c ", 1, nil); s != "" {
		t.Fatal("empty facts should dump no union fields", s)
	}
	facts := []*FactUnion{MakeFactUnion(uv, 0)}
	out := uv.OutputValueDump("c ", 1, facts)
	if !strings.Contains(out, "g_u.f0") {
		t.Fatal(out)
	}
	if strings.Contains(out, "g_u.f1") {
		t.Fatal("f1 unreadable", out)
	}
}

func TestOutputValueDumpArrayExpand(t *testing.T) {
	v := &Variable{
		Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2},
		Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
	}
	out := v.OutputValueDump("c ", 1, nil)
	if !strings.Contains(out, "g_a[0]") || !strings.Contains(out, "g_a[1]") {
		t.Fatal(out)
	}
}

func TestExpandWithinRanges(t *testing.T) {
	got := expandWithinRanges([]int{2, 2})
	if len(got) != 4 {
		t.Fatal(len(got))
	}
}

func TestBlindCheckGlobalMain(t *testing.T) {
	opts := Defaults()
	opts.BlindCheckGlobal = true
	opts.MaxFuncs = 1
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	g := NewProgramGenerator(opts)
	g.GenerateAllTypes()
	// seed simple global
	v := CreateVariableScalars("g_x", GetIntType(), false, false)
	g.VS.GlobalList = []*Variable{v}
	g.GenerateFunctions()
	main := g.OutputMain()
	if !strings.Contains(main, "checksum g_x") {
		t.Fatal(main)
	}
	// no platform_main / crc in blind path
	if strings.Contains(main, "platform_main_begin") {
		t.Fatal("blind path should skip platform begin", main)
	}
}
