// Upstream: util.cpp / util.h (gensym, reset_gensym, output helpers, permute).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"strings"
)

// GenSym is util.cpp gensym_count state. Session VariableSelector owns a GenSym;
// package currentSession().GenSym mirrors the process-wide counter when no session is injected.
type GenSym struct {
	count int
}

// Gensym mirrors util.cpp gensym(basename) on the session counter.
func Gensym(basename string) string {
	return currentSession().GenSym.Next(basename)
}

// ResetDefaultGensym mirrors reset_gensym on the session counter.
func ResetDefaultGensym() {
	currentSession().GenSym.Reset()
}

// Reset mirrors reset_gensym.
func (g *GenSym) Reset() {
	if g != nil {
		g.count = 0
	}
}

// Next mirrors gensym(basename): basename + (++count).
// util.cpp: ss << basename; ss << ++gensym_count;
// Nil receiver uses session GenSym (not a one-shot local counter).
// empty basename is broken IR sticky — no invent bare "1"/"2" numeric names
func (g *GenSym) Next(basename string) string {
	if g == nil {
		return Gensym(basename)
	}
	if basename == "" {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	g.count++
	return fmt.Sprintf("%s%d", basename, g.count)
}

// LogAnalysisFail mirrors util.cpp log_analysis_fail.
// util.cpp:76–79 — append to errlog; always returns false.
func LogAnalysisFail(msg string) bool {
	b := &currentSession().AnalysisErrLog
	b.WriteString("Analysis failed at ")
	b.WriteString(msg)
	b.WriteString("\n")
	return false
}

// AnalysisErrLog returns the accumulated analysis-fail log (tests / debug).
func AnalysisErrLog() string { return currentSession().AnalysisErrLog.String() }

// ClearAnalysisErrLog resets util errlog (finalization / tests).
func ClearAnalysisErrLog() { currentSession().AnalysisErrLog.Reset() }

// OutputPrintStr mirrors util.cpp output_print_str.
// util.cpp:146–157 — indent + printf("str"[, value]);
func OutputPrintStr(str, strValue string, indent int) string {
	var b strings.Builder
	b.WriteString(OutputTab(indent))
	b.WriteString("printf(\"")
	b.WriteString(str)
	b.WriteString("\"")
	if strValue != "" {
		b.WriteString(", ")
		b.WriteString(strValue)
	}
	b.WriteString(");")
	return b.String()
}

// OutputOpenEncloser mirrors util.cpp output_open_encloser.
// util.cpp:159–164 — tab + symbol + newline; indent++.
// Returns emitted text and new indent.
func OutputOpenEncloser(symbol string, indent int) (out string, newIndent int) {
	return OutputTab(indent) + symbol + "\n", indent + 1
}

// OutputCloseEncloser mirrors util.cpp output_close_encloser.
// util.cpp:166–174 — optional newline, indent--, tab + symbol.
func OutputCloseEncloser(symbol string, indent int, noNewline bool) (out string, newIndent int) {
	newIndent = indent - 1
	if newIndent < 0 {
		// incomplete indent sticky — fail closed clamp (no invent negative indent)
		sessNoteError(nil, ErrGeneric)
		newIndent = 0
	}
	var b strings.Builder
	if !noNewline {
		b.WriteString("\n")
	}
	b.WriteString(OutputTab(newIndent))
	b.WriteString(symbol)
	return b.String(), newIndent
}

// PermuteInts mirrors util.cpp permute for an integer slice.
// util.cpp:85–104 — all next_permutation orderings starting from sorted input order.
// Empty input soft empty; incomplete handled by caller.
func PermuteInts(in []int) [][]int {
	if len(in) == 0 {
		return nil
	}
	// work on a copy; first entry is current order (C++ pushes `in` then next_permutation)
	cur := append([]int(nil), in...)
	out := [][]int{append([]int(nil), cur...)}
	if len(cur) == 1 {
		return out
	}
	// lexicographic next_permutation until wrapped
	for nextPermutation(cur) {
		out = append(out, append([]int(nil), cur...))
	}
	return out
}

// nextPermutation is std::next_permutation for int slices (lexicographic).
func nextPermutation(a []int) bool {
	// find rightmost a[i] < a[i+1]
	i := len(a) - 2
	for i >= 0 && a[i] >= a[i+1] {
		i--
	}
	if i < 0 {
		return false
	}
	// find rightmost a[j] > a[i]
	j := len(a) - 1
	for a[j] <= a[i] {
		j--
	}
	a[i], a[j] = a[j], a[i]
	// reverse suffix
	for l, r := i+1, len(a)-1; l < r; l, r = l+1, r-1 {
		a[l], a[r] = a[r], a[l]
	}
	return true
}
