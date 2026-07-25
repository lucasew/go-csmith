package csmith

import "testing"

func TestGetIntSubfieldNamesNested(t *testing.T) {
	// S1 { int f0, f1 }; S2 { S1 f0; int f1 }
	s1 := &Type{isStruct: true, StructName: "S1", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	s2 := &Type{isStruct: true, StructName: "S2", Fields: []StructField{
		{Name: "f0", Type: s1, BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	got := s2.GetIntSubfieldNames("", nil)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("sticky")
	}
	want := []string{".f0.f0", ".f0.f1", ".f1"}
	if len(got) != 3 {
		t.Fatalf("got %#v", got)
	}
	for i, w := range want {
		if got[i].Name != w {
			t.Fatalf("[%d] got %q want %q", i, got[i].Name, w)
		}
	}
}
