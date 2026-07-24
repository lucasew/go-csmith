// Upstream: PartialExpander.h / PartialExpander.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// PartialExpander gates which statement kinds may be generated when --partial-expand is set.
// When inactive (MAX key false), ExpandCheck always allows all kinds.


// InitPartialExpander mirrors PartialExpander::init_partial_expander.
// PartialExpander.cpp:59–67 — parse comma-separated kind names; enable MAX sentinel.
// Empty options fail closed false (C++ parse_options("") → set_expand("") false).
func InitPartialExpander(options string) bool {
	currentSession().PartialExpands = initPartialMap(false)
	if !parsePartialOptions(options, ',') {
		// leave currentSession().PartialExpands half-init; callers treat false as fail
		return false
	}
	// MAX_STATEMENT_TYPE sentinel means "partial mode active"
	currentSession().PartialExpands[MaxStatementType] = true
	currentSession().PartialExpandsBackup = copyPartialMap(currentSession().PartialExpands)
	return true
}

func initPartialMap(value bool) map[StatementType]bool {
	m := map[StatementType]bool{
		StmtAssign:       value,
		StmtBlock:        value,
		StmtFor:          value,
		StmtIfElse:       value,
		StmtInvoke:       value,
		StmtReturn:       value,
		MaxStatementType: value,
	}
	return m
}

func copyPartialMap(src map[StatementType]bool) map[StatementType]bool {
	if src == nil {
		return nil
	}
	dest := make(map[StatementType]bool, len(src))
	for k, v := range src {
		dest[k] = v
	}
	return dest
}

func parsePartialOptions(options string, sep byte) bool {
	// PartialExpander.cpp:71–86 — empty string yields one empty token → set_expand fails
	if options == "" {
		return false
	}
	parts := SplitString(options, sep)
	if len(parts) == 0 {
		// single token without separator (SplitString emptied)
		parts = []string{options}
	}
	for _, s := range parts {
		if !setPartialExpand(s) {
			return false
		}
	}
	return true
}

func setPartialExpand(s string) bool {
	switch s {
	case "assignment":
		currentSession().PartialExpands[StmtAssign] = true
	case "block":
		currentSession().PartialExpands[StmtBlock] = true
	case "for":
		currentSession().PartialExpands[StmtFor] = true
	case "if-else":
		currentSession().PartialExpands[StmtIfElse] = true
	case "invoke":
		currentSession().PartialExpands[StmtInvoke] = true
	case "return":
		currentSession().PartialExpands[StmtReturn] = true
	case "all":
		currentSession().PartialExpands = initPartialMap(true)
	default:
		return false
	}
	return true
}

// SetStmtExpand mirrors PartialExpander::set_stmt_expand.
func SetStmtExpand(t StatementType, value bool) {
	if currentSession().PartialExpands == nil {
		currentSession().PartialExpands = initPartialMap(false)
	}
	currentSession().PartialExpands[t] = value
}

// RestorePartialExpanderInitValues mirrors restore_init_values.
// PartialExpander.cpp:122–125.
func RestorePartialExpanderInitValues() {
	if currentSession().PartialExpandsBackup != nil {
		currentSession().PartialExpands = copyPartialMap(currentSession().PartialExpandsBackup)
	}
}

// DirectExpandCheck mirrors PartialExpander::direct_expand_check.
// PartialExpander.cpp:127–130.
func DirectExpandCheck(t StatementType) bool {
	if currentSession().PartialExpands == nil {
		return false
	}
	return currentSession().PartialExpands[t]
}

// ExpandCheck mirrors PartialExpander::expand_check.
// PartialExpander.cpp:132–151 — if partial mode off, allow all; else allow listed
// kinds (Assign also ok if Invoke listed); first success clears MAX sentinel.
func ExpandCheck(t StatementType) bool {
	if currentSession().PartialExpands == nil || !currentSession().PartialExpands[MaxStatementType] {
		// not in partial mode → all valid
		return true
	}
	// PartialExpander.cpp:137 — assert(expands_.find(t) != end)
	// map only holds Assign/Block/For/IfElse/Invoke/Return/MAX; other kinds fail closed
	// (filter rejects), not soft invent allow-all for unregistered types
	rv := currentSession().PartialExpands[t]
	if t == StmtAssign {
		// Assign also ok when Invoke is listed (PartialExpander.cpp:143–145)
		rv = rv || currentSession().PartialExpands[StmtInvoke]
	}
	if rv {
		// after first successful expand, disable further forcing
		currentSession().PartialExpands[MaxStatementType] = false
	}
	return rv
}

// ClearPartialExpander resets package state (tests / finalization).
func ClearPartialExpander() {
	currentSession().PartialExpands = nil
	currentSession().PartialExpandsBackup = nil
}

// InitPartialExpanderFromOptions wires CGOptions::partial_expand string.
func InitPartialExpanderFromOptions(opts Options) bool {
	if opts.PartialExpand == "" {
		ClearPartialExpander()
		return true
	}
	return InitPartialExpander(opts.PartialExpand)
}
