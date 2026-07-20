package csmith

import "testing"

// CHECKLIST: Error.cpp::* — Error process sticky codes

func TestErrorCodesMatchUpstream(t *testing.T) {
	// Error.h macros
	if ErrSuccess != 0 || ErrGeneric != -1 || ErrExceedMaxDepth != -2 ||
		ErrFilter != -3 || ErrBacktracking != -4 || ErrCompatibleCheck != -5 ||
		ErrInvalidSimpleDelta != -6 {
		t.Fatalf("error codes mismatch: success=%d generic=%d depth=%d filter=%d bt=%d compat=%d delta=%d",
			ErrSuccess, ErrGeneric, ErrExceedMaxDepth, ErrFilter, ErrBacktracking, ErrCompatibleCheck, ErrInvalidSimpleDelta)
	}
}

func TestErrorGetSetClear(t *testing.T) {
	// Error::set_error / get_error; ClearError → SUCCESS
	ClearError()
	if GetError() != ErrSuccess || HasError() {
		t.Fatal("ClearError must yield SUCCESS")
	}
	SetError(ErrGeneric)
	if GetError() != ErrGeneric || !HasError() {
		t.Fatal("SetError(ERROR) must sticky")
	}
	SetError(ErrExceedMaxDepth)
	if GetError() != ErrExceedMaxDepth {
		t.Fatalf("overwrite set: got %d", GetError())
	}
	ClearError()
	if GetError() != ErrSuccess {
		t.Fatal("ClearError after sticky")
	}
}
