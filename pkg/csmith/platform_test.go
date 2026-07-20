package csmith

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlatformGenSeedNonZeroUsually(t *testing.T) {
	// platform_gen_seed — not a fixed invent seed
	a := PlatformGenSeed()
	b := PlatformGenSeed()
	// Extremely unlikely both zero from crypto/rand
	if a == 0 && b == 0 {
		t.Fatal("both seeds zero")
	}
}

func TestCreateDir(t *testing.T) {
	// platform.cpp:119–128 create_dir
	dir := filepath.Join(t.TempDir(), "out")
	if !CreateDir(dir) {
		t.Fatal("create")
	}
	if !CreateDir(dir) {
		t.Fatal("EEXIST must succeed")
	}
	// nested missing parent → false (unlike MkdirAll)
	nested := filepath.Join(dir, "missing", "deep")
	if CreateDir(nested) {
		t.Fatal("no parent create")
	}
	if CreateDir("") {
		t.Fatal("empty")
	}
	_ = os.RemoveAll(dir)
}
