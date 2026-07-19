// Upstream: VariableSelector.cpp OutputGlobalVariables / OutputVariableList paths.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"sort"
	"strings"
)

// OutputVariableList mirrors OutputVariableList for a slice of variables.
// VariableSelector.cpp / Variable.cpp — OutputDef per var; sorted names for determinism.
// Incomplete Variable* list fails closed sticky empty (no invent skip holes / partial section).
func OutputVariableList(vars []*Variable, indent string, forceStatic bool) string {
	if len(vars) == 0 {
		return ""
	}
	// incomplete list fails closed sticky before sort invents nil-first ordering
	if !VariablesComplete(vars) {
		SetError(ErrGeneric)
		return ""
	}
	// stable order by name
	cp := append([]*Variable(nil), vars...)
	sort.SliceStable(cp, func(i, j int) bool {
		return cp[i].Name < cp[j].Name
	})
	var b strings.Builder
	for _, v := range cp {
		// OutputDef always live; no invent indent-only / blank lines for incomplete IR
		var def string
		if v.IsArray && v.AsArray != nil {
			def = v.AsArray.OutputDef()
		} else {
			def = v.OutputDef(forceStatic)
		}
		if def == "" {
			return ""
		}
		b.WriteString(indent)
		b.WriteString(def)
		if !strings.HasSuffix(def, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// OutputGlobalVariables mirrors OutputGlobalVariables.
// VariableSelector.cpp:1594–1601 — comment header + list (no access_once toggle).
// no invent section header without any live global defs
func OutputGlobalVariables(vars []*Variable) string {
	body := OutputVariableList(vars, "", true)
	if body == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(OutputCommentLine("--- GLOBAL VARIABLES ---", false, false))
	b.WriteString(body)
	return b.String()
}

// OutputGlobalVariablesDecls mirrors OutputGlobalVariablesDecls with optional prefix.
// VariableSelector.cpp:1604–1612.
// no invent section header without any live decls
func OutputGlobalVariablesDecls(vars []*Variable, prefix string) string {
	body := OutputVariableList(vars, "", false)
	if body == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(OutputCommentLine("--- GLOBAL VARIABLES ---", false, false))
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
