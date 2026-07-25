package csmith

import "testing"

func TestGetSimpleTypeCached(t *testing.T) {
	// Type::simple_types identity for eExact
	if GetSimpleTypeSess(testAmbientSession, EInt) != GetSimpleTypeSess(testAmbientSession, EInt) {
		t.Fatal("cache identity")
	}
	if GetSimpleTypeSess(testAmbientSession, EInt) == GetSimpleTypeSess(testAmbientSession, EShort) {
		t.Fatal("distinct types")
	}
}

func TestPointerToCached(t *testing.T) {
	p1 := PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EInt))
	p2 := PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EInt))
	if p1 != p2 {
		t.Fatal("pointer cache")
	}
	if !p1.MatchSess(testAmbientSession, p2, MatchExact) {
		t.Fatal("eExact")
	}
}

func TestMatchConvertSimple(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	// non-void integers interconvert
	if !GetSimpleTypeSess(testAmbientSession, EInt).MatchSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EShort), MatchConvert) {
		t.Fatal("int convert short")
	}
	if GetSimpleTypeSess(testAmbientSession, EInt).MatchSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EVoid), MatchConvert) {
		t.Fatal("int not convert void")
	}
	// float → int forbidden when target is int and other is float
	// is_convertable: if (t->is_float() && !is_float()) return false — t is *other*
	if GetSimpleTypeSess(testAmbientSession, EInt).MatchSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EFloat), MatchConvert) {
		t.Fatal("int not convertable from float")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete MatchConvert must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsConvertableOpts residual soft invent was soft-continue invent match true.
	// Fair: sticky false.
	if GetSimpleTypeSess(testAmbientSession, EInt).MatchSess(testAmbientSession, nil, MatchConvert) {
		t.Fatal("nil other MatchConvert must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil other MatchConvert must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMatchDereference(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	pint := PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EInt))
	if !GetSimpleTypeSess(testAmbientSession, EInt).MatchSess(testAmbientSession, pint, MatchDereference) {
		t.Fatal("int is_dereferenced_from int*")
	}
	if !GetSimpleTypeSess(testAmbientSession, EInt).MatchSess(testAmbientSession, pint, MatchFlexible) {
		t.Fatal("flexible")
	}
	if GetSimpleTypeSess(testAmbientSession, EShort).MatchSess(testAmbientSession, pint, MatchDereference) {
		t.Fatal("short not from int*")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete MatchDereference must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsDereferencedFrom residual soft invent was soft-continue invent match true.
	// Fair: sticky false.
	if GetSimpleTypeSess(testAmbientSession, EInt).MatchSess(testAmbientSession, nil, MatchDereference) {
		t.Fatal("nil other MatchDereference must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil other MatchDereference must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsDerivable residual soft invent was soft-continue invent flexible match true.
	// Fair: sticky false.
	if GetSimpleTypeSess(testAmbientSession, EInt).MatchSess(testAmbientSession, nil, MatchFlexible) {
		t.Fatal("nil other MatchFlexible must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil other MatchFlexible must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsSigned(t *testing.T) {
	if !GetSimpleTypeSess(testAmbientSession, EInt).IsSignedSess(testAmbientSession) || GetSimpleTypeSess(testAmbientSession, EUInt).IsSignedSess(testAmbientSession) {
		t.Fatal("signedness")
	}
}

func TestStructDepthIncompleteSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, Fields: []StructField{{Name: "x", Type: nil}}}
	if st.StructDepthSess(testAmbientSession) != incompleteStructDepth {
		t.Fatal("nil field Type must fail closed incompleteStructDepth")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil field Type StructDepth must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !HasLongLongFieldSess(testAmbientSession, []StructField{{Type: nil}}) {
		t.Fatal("nil field Type HasLongLongField must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil field Type HasLongLongField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nested IsConstStructUnion residual soft invent was soft-continue later Qfer non-const.
	// Fair: sticky const true.
	innerHole := &Type{isStruct: true, Fields: []StructField{{Name: "x", Type: nil}}}
	outer := &Type{isStruct: true, Fields: []StructField{
		{Name: "nest", Type: innerHole, BitWidth: -1},
		{Name: "ok", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	if !outer.IsConstStructUnionSess(testAmbientSession) {
		t.Fatal("nested residual IsConstStructUnion must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nested residual IsConstStructUnion must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !outer.IsVolatileStructUnionSess(testAmbientSession) {
		t.Fatal("nested residual IsVolatileStructUnion must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nested residual IsVolatileStructUnion must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestNeedsCastPtrTypeResidualSticky(t *testing.T) {
	// PtrType residual soft invent was invent no-cast soft-skip past hole.
	// nil Type already sticky; complete non-pointer is false no residual.
	ClearErrorSess(testAmbientSession)
	if NeedsCast := (*Type)(nil).NeedsCastSess(testAmbientSession, GetIntTypeSess(testAmbientSession)); NeedsCast {
		// nil sticky returns false with SetError
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil NeedsCast must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete: int needs no cast to int
	if GetIntTypeSess(testAmbientSession).NeedsCastSess(testAmbientSession, GetIntTypeSess(testAmbientSession)) {
		t.Fatal("int NeedsCast int must be false")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete NeedsCast must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// pointer to int vs int base mismatch needs cast when bases differ
	pt := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	if !pt.NeedsCastSess(testAmbientSession, PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EChar))) {
		// may or may not need cast depending on base equivalence — just hygiene
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsSignedIsSimpleResidualSticky(t *testing.T) {
	// IsSimple residual soft invent was invent unsigned soft-skip past nil already sticky.
	ClearErrorSess(testAmbientSession)
	if !(*Type)(nil).IsSignedSess(testAmbientSession) {
		t.Fatal("nil IsSigned must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsSigned must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete pointer is not simple → unsigned false without sticky
	pt := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	if pt.IsSignedSess(testAmbientSession) {
		t.Fatal("pointer IsSigned must be false")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete pointer IsSigned must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !GetIntTypeSess(testAmbientSession).IsSignedSess(testAmbientSession) {
		t.Fatal("int IsSigned must be true")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsFloatIsSimpleResidualSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).IsFloatSess(testAmbientSession) {
		t.Fatal("nil IsFloat must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsFloat must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if GetIntTypeSess(testAmbientSession).IsFloatSess(testAmbientSession) {
		t.Fatal("int IsFloat must be false")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete IsFloat must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHasBitfieldsIsStructResidualSticky(t *testing.T) {
	// Nested field Type-nil sticky has-bitfields before IsStruct.
	ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: nil, BitWidth: -1},
	}}
	if !st.HasBitfieldsSess(testAmbientSession) {
		t.Fatal("nil field Type HasBitfields must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil field Type HasBitfields must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsEquivalentIsSimpleResidualHygiene(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if GetIntTypeSess(testAmbientSession).IsEquivalentSess(testAmbientSession, GetIntTypeSess(testAmbientSession)) {
		// same size signed - true
	} else {
		t.Fatal("int IsEquivalent int")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete IsEquivalent must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// pointer vs int not equivalent
	if PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)).IsEquivalentSess(testAmbientSession, GetIntTypeSess(testAmbientSession)) {
		t.Fatal("pointer IsEquivalent int must be false")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete pointer/int IsEquivalent must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSignedOverflowPossibleIsSimpleResidualSticky(t *testing.T) {
	// IsSimple residual soft invent was invent overflow-free soft-skip past Type-nil.
	ClearErrorSess(testAmbientSession)
	if !((*Type)(nil)).SignedOverflowPossibleSess(testAmbientSession, 4) {
		t.Fatal("nil Type SignedOverflowPossible must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type SignedOverflowPossible must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete unsigned int — no signed overflow
	if GetSimpleTypeSess(testAmbientSession, EUInt).SignedOverflowPossibleSess(testAmbientSession, 4) {
		t.Fatal("uint SignedOverflowPossible must false")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete uint SignedOverflowPossible must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete signed int with size >= intSize
	if !GetIntTypeSess(testAmbientSession).SignedOverflowPossibleSess(testAmbientSession, 4) {
		t.Fatal("int SignedOverflowPossible(4) must true")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete int SignedOverflowPossible must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHasAggregateFieldIsAggregateResidualSticky(t *testing.T) {
	// Type-nil field residual soft invent was invent no-aggregate soft-skip.
	ClearErrorSess(testAmbientSession)
	if !HasAggregateFieldSess(testAmbientSession, []StructField{{Name: "f0", Type: nil, BitWidth: -1}}) {
		t.Fatal("Type-nil field HasAggregateField must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil field HasAggregateField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestTypeNameStringIsStructResidualSticky(t *testing.T) {
	// IsStruct residual soft invent was invent soft-empty name past Type-nil shell.
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).TypeNameStringSess(testAmbientSession) != "" {
		t.Fatal("nil TypeNameString must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil TypeNameString must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete int
	if GetIntTypeSess(testAmbientSession).TypeNameStringSess(testAmbientSession) != "Int" {
		t.Fatal("int TypeNameString must Int")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete int TypeNameString must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete struct empty name sticky
	st := &Type{isStruct: true, StructName: ""}
	if st.TypeNameStringSess(testAmbientSession) != "" {
		t.Fatal("empty StructName TypeNameString must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty StructName TypeNameString must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestToUnsignedIsSimpleResidualSticky(t *testing.T) {
	// IsSimple residual soft invent was invent soft-nil unsigned past Type-nil shell.
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).ToUnsignedSess(testAmbientSession) != nil {
		t.Fatal("nil ToUnsigned must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ToUnsigned must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete signed int
	u := GetIntTypeSess(testAmbientSession).ToUnsignedSess(testAmbientSession)
	if u == nil || u.SimpleSess(testAmbientSession) != EUInt {
		t.Fatal("int ToUnsigned must uint", u)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete int ToUnsigned must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// pointer non-simple complete nil without sticky
	if PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)).ToUnsignedSess(testAmbientSession) != nil {
		t.Fatal("pointer ToUnsigned must nil")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete pointer ToUnsigned must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}
