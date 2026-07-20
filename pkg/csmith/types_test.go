package csmith

import "testing"

func TestGetSimpleTypeCached(t *testing.T) {
	// Type::simple_types identity for eExact
	if GetSimpleType(EInt) != GetSimpleType(EInt) {
		t.Fatal("cache identity")
	}
	if GetSimpleType(EInt) == GetSimpleType(EShort) {
		t.Fatal("distinct types")
	}
}

func TestPointerToCached(t *testing.T) {
	p1 := PointerTo(GetSimpleType(EInt))
	p2 := PointerTo(GetSimpleType(EInt))
	if p1 != p2 {
		t.Fatal("pointer cache")
	}
	if !p1.Match(p2, MatchExact) {
		t.Fatal("eExact")
	}
}

func TestMatchConvertSimple(t *testing.T) {
	ClearError()
	// non-void integers interconvert
	if !GetSimpleType(EInt).Match(GetSimpleType(EShort), MatchConvert) {
		t.Fatal("int convert short")
	}
	if GetSimpleType(EInt).Match(GetSimpleType(EVoid), MatchConvert) {
		t.Fatal("int not convert void")
	}
	// float → int forbidden when target is int and other is float
	// is_convertable: if (t->is_float() && !is_float()) return false — t is *other*
	if GetSimpleType(EInt).Match(GetSimpleType(EFloat), MatchConvert) {
		t.Fatal("int not convertable from float")
	}
	if HasError() {
		t.Fatal("complete MatchConvert must not sticky")
	}
	ClearError()
	// IsConvertableOpts residual soft invent was soft-continue invent match true.
	// Fair: sticky false.
	if GetSimpleType(EInt).Match(nil, MatchConvert) {
		t.Fatal("nil other MatchConvert must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil other MatchConvert must SetError sticky")
	}
	ClearError()
}

func TestMatchDereference(t *testing.T) {
	ClearError()
	pint := PointerTo(GetSimpleType(EInt))
	if !GetSimpleType(EInt).Match(pint, MatchDereference) {
		t.Fatal("int is_dereferenced_from int*")
	}
	if !GetSimpleType(EInt).Match(pint, MatchFlexible) {
		t.Fatal("flexible")
	}
	if GetSimpleType(EShort).Match(pint, MatchDereference) {
		t.Fatal("short not from int*")
	}
	if HasError() {
		t.Fatal("complete MatchDereference must not sticky")
	}
	ClearError()
	// IsDereferencedFrom residual soft invent was soft-continue invent match true.
	// Fair: sticky false.
	if GetSimpleType(EInt).Match(nil, MatchDereference) {
		t.Fatal("nil other MatchDereference must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil other MatchDereference must SetError sticky")
	}
	ClearError()
	// IsDerivable residual soft invent was soft-continue invent flexible match true.
	// Fair: sticky false.
	if GetSimpleType(EInt).Match(nil, MatchFlexible) {
		t.Fatal("nil other MatchFlexible must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil other MatchFlexible must SetError sticky")
	}
	ClearError()
}

func TestIsSigned(t *testing.T) {
	if !GetSimpleType(EInt).IsSigned() || GetSimpleType(EUInt).IsSigned() {
		t.Fatal("signedness")
	}
}

func TestStructDepthIncompleteSticky(t *testing.T) {
	ClearError()
	st := &Type{isStruct: true, Fields: []StructField{{Name: "x", Type: nil}}}
	if st.StructDepth() != incompleteStructDepth {
		t.Fatal("nil field Type must fail closed incompleteStructDepth")
	}
	if !HasError() {
		t.Fatal("nil field Type StructDepth must SetError sticky")
	}
	ClearError()
	if !HasLongLongField([]StructField{{Type: nil}}) {
		t.Fatal("nil field Type HasLongLongField must fail closed true")
	}
	if !HasError() {
		t.Fatal("nil field Type HasLongLongField must SetError sticky")
	}
	ClearError()
	// nested IsConstStructUnion residual soft invent was soft-continue later Qfer non-const.
	// Fair: sticky const true.
	innerHole := &Type{isStruct: true, Fields: []StructField{{Name: "x", Type: nil}}}
	outer := &Type{isStruct: true, Fields: []StructField{
		{Name: "nest", Type: innerHole, BitWidth: -1},
		{Name: "ok", Type: GetIntType(), BitWidth: -1},
	}}
	if !outer.IsConstStructUnion() {
		t.Fatal("nested residual IsConstStructUnion must fail closed true")
	}
	if !HasError() {
		t.Fatal("nested residual IsConstStructUnion must SetError sticky")
	}
	ClearError()
	if !outer.IsVolatileStructUnion() {
		t.Fatal("nested residual IsVolatileStructUnion must fail closed true")
	}
	if !HasError() {
		t.Fatal("nested residual IsVolatileStructUnion must SetError sticky")
	}
	ClearError()
}
