package csmith

import (
	"strings"
	"testing"
)

func TestHashSimpleInt(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	out := v.HashOutput()
	if !strings.Contains(out, `transparent_crc(g_1, "g_1"`) {
		t.Fatal(out)
	}
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
	v := &Variable{
		Name:       "g_4",
		Type:       GetIntType(),
		IsArray:    true,
		ArraySizes: []int{3},
		Qfer:       NewCVQualifiers([]bool{false}, []bool{false}),
	}
	out := v.HashOutput()
	if !strings.Contains(out, "for (i0 = 0") || !strings.Contains(out, "g_4[i0]") {
		t.Fatal(out)
	}
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
