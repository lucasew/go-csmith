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
	ClearErrorSess(testAmbientSession)
	if GetErrorSess(testAmbientSession) != ErrSuccess || HasErrorSess(testAmbientSession) {
		t.Fatal("ClearError must yield SUCCESS")
	}
	SetErrorSess(testAmbientSession, ErrGeneric)
	if GetErrorSess(testAmbientSession) != ErrGeneric || !HasErrorSess(testAmbientSession) {
		t.Fatal("SetErrorSess(testAmbientSession, ERROR) must sticky")
	}
	SetErrorSess(testAmbientSession, ErrExceedMaxDepth)
	if GetErrorSess(testAmbientSession) != ErrExceedMaxDepth {
		t.Fatalf("overwrite set: got %d", GetErrorSess(testAmbientSession))
	}
	ClearErrorSess(testAmbientSession)
	if GetErrorSess(testAmbientSession) != ErrSuccess {
		t.Fatal("ClearError after sticky")
	}
}
