// Upstream: StringUtils.h / StringUtils.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"strconv"
	"strings"
	"unicode"
)

// SplitString mirrors StringUtils::split_string(str, v, sep_char).
// StringUtils.cpp:99–114 — ignore spaces; skip empty tokens.
func SplitString(str string, sep byte) []string {
	var v []string
	pos := 0
	for {
		// ignore_spaces
		for pos < len(str) && unicode.IsSpace(rune(str[pos])) {
			pos++
		}
		if pos >= len(str) {
			break
		}
		start := pos
		// find sep
		for pos < len(str) && str[pos] != sep {
			pos++
		}
		s := str[start:pos]
		if s != "" {
			v = append(v, s)
		}
		if pos >= len(str) {
			break
		}
		pos++ // skip sep
	}
	return v
}

// SplitStringAny mirrors StringUtils::split_string with multi-char separators.
// StringUtils.cpp:116–131.
func SplitStringAny(str, seps string) []string {
	var v []string
	pos := 0
	for {
		for pos < len(str) && unicode.IsSpace(rune(str[pos])) {
			pos++
		}
		if pos >= len(str) {
			break
		}
		start := pos
		for pos < len(str) && !strings.ContainsRune(seps, rune(str[pos])) {
			pos++
		}
		s := str[start:pos]
		if s != "" {
			v = append(v, s)
		}
		if pos >= len(str) {
			break
		}
		pos++
	}
	return v
}

// GetSubstring mirrors StringUtils::get_substring — content between open/close.
// StringUtils.cpp:55–68.
func GetSubstring(s string, open, close byte) string {
	if s == "" {
		return ""
	}
	pos := 0
	for pos < len(s) && unicode.IsSpace(rune(s[pos])) {
		pos++
	}
	if pos >= len(s) || s[pos] != open {
		return ""
	}
	pos++
	end := strings.IndexByte(s[pos:], close)
	if end < 0 {
		return ""
	}
	return s[pos : pos+end]
}

// FirstNonSpaceChar mirrors StringUtils::first_nonspace_char.
func FirstNonSpaceChar(s string) byte {
	for i := 0; i < len(s); i++ {
		if !unicode.IsSpace(rune(s[i])) {
			return s[i]
		}
	}
	return 0
}

// Int2Str mirrors StringUtils::int2str.
func Int2Str(n int) string {
	return strconv.Itoa(n)
}
