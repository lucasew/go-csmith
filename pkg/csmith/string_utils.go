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

// EmptyLine mirrors StringUtils::empty_line.
// StringUtils.cpp:39–44.
func EmptyLine(line string) bool {
	if line == "" {
		return true
	}
	return strings.TrimSpace(line) == ""
}

// IsSpaceChar mirrors StringUtils::is_space.
// StringUtils.cpp:46–48.
func IsSpaceChar(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n'
}

// Str2Int mirrors StringUtils::str2int — strip outer parens; hex 0x prefix.
// StringUtils.cpp:151–165.
func Str2Int(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return -1
	}
	// StringUtils.cpp:152–155 — if starts '(' assert ends ')'; strip one layer at a time
	for len(s) > 0 && s[0] == '(' {
		// assert(s[s.length() - 1] == ')'); no soft invent parse without close
		if len(s) < 2 || s[len(s)-1] != ')' {
			return -1
		}
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		n, err := strconv.ParseInt(s[2:], 16, 64)
		if err != nil {
			return -1
		}
		return int(n)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}

// Str2Int64 mirrors StringUtils::str2longlong.
// StringUtils.cpp:173–193.
func Str2Int64(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		var i int64
		for j := 2; j < len(s); j++ {
			c := s[j]
			var v int64
			switch {
			case c >= '0' && c <= '9':
				v = int64(c - '0')
			case c >= 'A' && c <= 'F':
				v = 10 + int64(c-'A')
			case c >= 'a' && c <= 'f':
				v = 10 + int64(c-'a')
			default:
				return i
			}
			i = i*16 + v
		}
		return i
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// Int642Str mirrors StringUtils::longlong2str.
func Int642Str(n int64) string {
	return strconv.FormatInt(n, 10)
}

// Chop mirrors StringUtils::chop — trim leading/trailing spaces and tabs.
// StringUtils.cpp:200–208.
func Chop(str string) string {
	return strings.Trim(str, " \t")
}

// EndWith mirrors StringUtils::end_with.
func EndWith(s, tail string) bool {
	return strings.HasSuffix(s, tail)
}

// SplitIntString mirrors StringUtils::split_int_string.
// StringUtils.cpp:133–148.
func SplitIntString(str, seps string) []int {
	parts := SplitStringAny(str, seps)
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		out = append(out, Str2Int(p))
	}
	return out
}

// BreakupAssigns mirrors StringUtils::breakup_assigns.
// StringUtils.cpp:214–227 — "a=1; b=2" → vars/values pairs (split on ';' then '=').
// StringUtils.cpp:222 — assert(pair.size() == 2); malformed stmt fails whole parse.
func BreakupAssigns(assigns string) (vars, values []string) {
	stmts := SplitString(assigns, ';')
	for _, st := range stmts {
		st = Chop(st)
		if st == "" {
			continue
		}
		// C++ split_string on '='; assert exactly two parts
		pair := SplitString(st, '=')
		if len(pair) != 2 {
			// fail closed — no soft invent skip of broken assignment
			return nil, nil
		}
		vars = append(vars, Chop(pair[0]))
		values = append(values, Chop(pair[1]))
	}
	return vars, values
}
