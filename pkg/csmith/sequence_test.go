package csmith

import "testing"

func TestLinearSequence(t *testing.T) {
	// LinearSequence.cpp:45–81
	ClearErrorSess(testAmbientSession)
	s := NewLinearSequence(LinearSequenceDefaultSep)
	s.InitSequence()
	if s.SequenceLength() != 0 {
		t.Fatal("empty")
	}
	// empty get_sequence sticky
	if s.GetSequence() != "" || !HasErrorSess(testAmbientSession) {
		t.Fatal("empty sequence sticky")
	}
	ClearErrorSess(testAmbientSession)
	s.AddNumber(3, 10, 0)
	s.AddNumber(1, 10, 1)
	s.AddNumber(7, 10, 2)
	if s.SequenceLength() != 3 {
		t.Fatal(s.SequenceLength())
	}
	if s.GetNumberByPos(1) != 1 {
		t.Fatal(s.GetNumberByPos(1))
	}
	if s.GetSequence() != "3_1_7" {
		t.Fatal(s.GetSequence())
	}
	if s.GetNumber(99) != -1 {
		t.Fatal("get_number always -1")
	}
	// missing pos sticky
	if s.GetNumberByPos(9) != -1 || !HasErrorSess(testAmbientSession) {
		t.Fatal("missing pos sticky")
	}
	ClearErrorSess(testAmbientSession)
	s.Clear()
	if s.SequenceLength() != 0 {
		t.Fatal("clear")
	}
	// nil sticky
	var nilS *LinearSequence
	nilS.AddNumber(1, 1, 0)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil AddNumber sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSequenceFactory(t *testing.T) {
	// SequenceFactory.cpp:44–59
	DestroySequences()
	s := MakeSequence()
	if s == nil || s.SepChar() != '_' {
		t.Fatal("default sep")
	}
	if CurrentSepCharSess(testAmbientSession) != '_' {
		t.Fatal(CurrentSepCharSess(testAmbientSession))
	}
	s.AddNumber(0, 1, 0)
	DestroySequences()
}

func TestParseSequenceLine(t *testing.T) {
	// SequenceLineParser.h:124–152
	got, ok := ParseSequenceLine("1_2_3", '_')
	if !ok || len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Fatal(got, ok)
	}
	// spaces
	got, ok = ParseSequenceLine(" 4, 5", ',')
	if !ok || len(got) != 2 || got[0] != 4 || got[1] != 5 {
		t.Fatal(got, ok)
	}
	// empty token fail
	if _, ok := ParseSequenceLine("", ','); ok {
		t.Fatal("empty must fail")
	}
	if _, ok := ParseSequenceLine("1,", ','); ok {
		t.Fatal("trailing sep fail")
	}
	if _, ok := ParseSequenceLine("a_b", '_'); ok {
		t.Fatal("non-int fail")
	}
}
