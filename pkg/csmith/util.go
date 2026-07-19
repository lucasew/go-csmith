// Upstream: util.cpp / util.h (gensym, reset_gensym).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "fmt"

// GenSym is util.cpp gensym_count state. Session VariableSelector owns a GenSym;
// package defaultGenSym mirrors the process-wide counter when no session is injected.
type GenSym struct {
	count int
}

// defaultGenSym is util.cpp gensym_count for callers that do not pass a GenSym
// (Block::create_new_tmp_var → gensym("t_") always hits the global counter).
var defaultGenSym GenSym

// Gensym mirrors util.cpp gensym(basename) on the package-wide counter.
func Gensym(basename string) string {
	return defaultGenSym.Next(basename)
}

// ResetDefaultGensym mirrors reset_gensym on the package counter.
func ResetDefaultGensym() {
	defaultGenSym.Reset()
}

// Reset mirrors reset_gensym.
func (g *GenSym) Reset() {
	if g != nil {
		g.count = 0
	}
}

// Next mirrors gensym(basename): basename + (++count).
// util.cpp: ss << basename; ss << ++gensym_count;
// Nil receiver uses package defaultGenSym (not a one-shot local counter).
// empty basename is broken IR sticky — no invent bare "1"/"2" numeric names
func (g *GenSym) Next(basename string) string {
	if g == nil {
		return Gensym(basename)
	}
	if basename == "" {
		SetError(ErrGeneric)
		return ""
	}
	g.count++
	return fmt.Sprintf("%s%d", basename, g.count)
}
