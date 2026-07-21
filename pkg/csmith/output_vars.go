// Upstream: VariableSelector.cpp OutputGlobalVariables / OutputVariableList paths.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"strings"
)

// OutputVariableList mirrors OutputVariableList for a slice of variables.
// Variable.cpp:855–864 — OutputDef per var in vector order (no invent name-sort);
// then for non-global lists OutputArrayInitializers (ctrl decl even when all brace-init).
// Incomplete Variable* list fails closed sticky empty (no invent skip holes / partial section).
func OutputVariableList(vars []*Variable, indent string, forceStatic bool) string {
	if len(vars) == 0 {
		return ""
	}
	// incomplete list fails closed sticky (no invent skip holes / partial section)
	if !VariablesComplete(vars) {
		SetError(ErrGeneric)
		return ""
	}
	// Variable.cpp:858–860 — iterate vars in given order (not name-sorted)
	var b strings.Builder
	for _, v := range vars {
		// OutputDef always live; sticky no invent indent-only / blank lines for incomplete IR
		// C++ static_cast ArrayVariable* when isArray; missing AsArray is broken IR
		// sticky (no invent scalar OutputDef for IsArray shell / soft re-pick partial list)
		if v.IsArray && v.AsArray == nil {
			SetError(ErrGeneric)
			return ""
		}
		var def string
		if v.IsArray && v.AsArray != nil {
			// ArrayVariable.cpp:478–479 — itemized (collective!=0) OutputDef is a no-op
			if v.AsArray.Collective != nil {
				continue
			}
			def = v.AsArray.OutputDef()
		} else {
			def = v.OutputDef(forceStatic)
		}
		// residual ERROR sticky — no invent soft-continue later vars past OutputDef residual
		if HasError() {
			return ""
		}
		if def == "" {
			SetError(ErrGeneric)
			return ""
		}
		b.WriteString(indent)
		b.WriteString(def)
		if !strings.HasSuffix(def, "\n") {
			b.WriteString("\n")
		}
	}
	// Variable.cpp:861–863 — if (!vars.empty() && !vars[0]->is_global())
	// OutputArrayInitializers: declares int i,j,k… whenever any array dim > 0,
	// even if every array uses brace init (no_loop_initializer true).
	if !vars[0].IsGlobal() {
		// residual ERROR sticky — no invent soft-skip initializers past IsGlobal residual
		if HasError() {
			return ""
		}
		inits := OutputArrayInitializers(vars, ProcessOptions(), indent)
		// residual ERROR sticky — no invent soft-return defs-only past OutputArrayInitializers residual
		if HasError() {
			return ""
		}
		b.WriteString(inits)
	} else if HasError() {
		// residual ERROR sticky — no invent soft-success past IsGlobal residual true
		return ""
	}
	return b.String()
}

// OutputGlobalVariables mirrors OutputGlobalVariables.
// VariableSelector.cpp:1594–1601 — comment header + list (no access_once toggle).
// no invent section header without any live global defs
func OutputGlobalVariables(vars []*Variable) string {
	body := OutputVariableList(vars, "", true)
	// residual ERROR sticky — no invent soft-header past OutputVariableList residual
	if HasError() {
		return ""
	}
	if body == "" {
		return ""
	}
	var b strings.Builder
	hdr := OutputCommentLine("--- GLOBAL VARIABLES ---", false, false)
	// residual ERROR sticky — no invent soft-body past OutputCommentLine residual
	if HasError() {
		return ""
	}
	b.WriteString(hdr)
	b.WriteString(body)
	return b.String()
}

// OutputGlobalVariablesDecls mirrors OutputGlobalVariablesDecls with optional prefix.
// VariableSelector.cpp:1604–1612.
// no invent section header without any live decls
func OutputGlobalVariablesDecls(vars []*Variable, prefix string) string {
	body := OutputVariableList(vars, "", false)
	// residual ERROR sticky — no invent soft-header past OutputVariableList residual
	if HasError() {
		return ""
	}
	if body == "" {
		return ""
	}
	var b strings.Builder
	hdr := OutputCommentLine("--- GLOBAL VARIABLES ---", false, false)
	// residual ERROR sticky — no invent soft-body past OutputCommentLine residual
	if HasError() {
		return ""
	}
	b.WriteString(hdr)
	// prefix each line (e.g. "extern ")
	if prefix == "" {
		b.WriteString(body)
		return b.String()
	}
	for _, line := range strings.Split(body, "\n") {
		if line == "" {
			continue
		}
		b.WriteString(prefix)
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}
