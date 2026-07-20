// Upstream: platform.h / platform.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"crypto/rand"
	"encoding/binary"
	"os"
	"runtime"
	"time"
)

// PlatformGenSeed mirrors platform_gen_seed.
// platform.cpp:97–103 — arc4random_buf when available; else read_time (rdtsc/time).
// Go: crypto/rand first; wall-clock fallback (no invent fixed seed 0).
func PlatformGenSeed() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		return binary.LittleEndian.Uint64(b[:])
	}
	// platform.cpp non-arc4 path → read_time / time(nullptr)
	return uint64(time.Now().UnixNano())
}

// CreateDir mirrors create_dir.
// platform.cpp:119–128 — mkdir(dir); true if created or already exists (EEXIST).
// Unlike MkdirAll, does not create parents.
func CreateDir(dir string) bool {
	if dir == "" {
		return false
	}
	err := os.Mkdir(dir, 0o770)
	if err == nil {
		return true
	}
	if os.IsExist(err) {
		return true
	}
	return false
}

// IsX8664 mirrors CGOptions::x86_64.
// CGOptions.cpp:579–585 — true when built for x86_64 host.
func IsX8664() bool {
	return runtime.GOARCH == "amd64"
}
