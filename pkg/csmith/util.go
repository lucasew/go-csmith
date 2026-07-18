// Upstream: util.cpp / util.h (gensym, reset_gensym).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "fmt"

// gensymCount is util.cpp gensym_count (process-wide in upstream; instance-local here for tests).
// VariableSelector and other ports use Gensym on a shared GenSym state.
type GenSym struct {
	count int
}

// Reset mirrors reset_gensym.
func (g *GenSym) Reset() {
	if g != nil {
		g.count = 0
	}
}

// Next mirrors gensym(basename): basename + (++count).
// util.cpp: ss << basename; ss << ++gensym_count;
func (g *GenSym) Next(basename string) string {
	if g == nil {
		g = &GenSym{}
	}
	g.count++
	return fmt.Sprintf("%s%d", basename, g.count)
}
