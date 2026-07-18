// Upstream: PartialExpander.h / PartialExpander.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// PartialExpander gates which statement kinds may be generated when --partial-expand is set.
// When inactive (MAX key false), ExpandCheck always allows all kinds.

var (
	partialExpands       map[StatementType]bool
	partialExpandsBackup map[StatementType]bool
)

// InitPartialExpander mirrors PartialExpander::init_partial_expander.
// PartialExpander.cpp:59–67 — parse comma-separated kind names; enable MAX sentinel.
func InitPartialExpander(options string) bool {
	partialExpands = initPartialMap(false)
	if options != "" {
		if !parsePartialOptions(options, ',') {
			return false
		}
	}
	// MAX_STATEMENT_TYPE sentinel means "partial mode active"
	partialExpands[MaxStatementType] = true
	partialExpandsBackup = copyPartialMap(partialExpands)
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
	parts := SplitString(options, sep)
	if len(parts) == 0 && options != "" {
		// single token without separator
		parts = []string{options}
	}
	// SplitString may skip empties; also handle raw single string
	if options != "" && len(parts) == 0 {
		parts = []string{options}
	}
	// re-split more carefully for no-spaces requirement
	if len(parts) == 0 {
		// empty options: nothing to enable
		return true
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
		partialExpands[StmtAssign] = true
	case "block":
		partialExpands[StmtBlock] = true
	case "for":
		partialExpands[StmtFor] = true
	case "if-else":
		partialExpands[StmtIfElse] = true
	case "invoke":
		partialExpands[StmtInvoke] = true
	case "return":
		partialExpands[StmtReturn] = true
	case "all":
		partialExpands = initPartialMap(true)
	default:
		return false
	}
	return true
}

// SetStmtExpand mirrors PartialExpander::set_stmt_expand.
func SetStmtExpand(t StatementType, value bool) {
	if partialExpands == nil {
		partialExpands = initPartialMap(false)
	}
	partialExpands[t] = value
}

// RestorePartialExpanderInitValues mirrors restore_init_values.
// PartialExpander.cpp:122–125.
func RestorePartialExpanderInitValues() {
	if partialExpandsBackup != nil {
		partialExpands = copyPartialMap(partialExpandsBackup)
	}
}

// DirectExpandCheck mirrors PartialExpander::direct_expand_check.
// PartialExpander.cpp:127–130.
func DirectExpandCheck(t StatementType) bool {
	if partialExpands == nil {
		return false
	}
	return partialExpands[t]
}

// ExpandCheck mirrors PartialExpander::expand_check.
// PartialExpander.cpp:132–151 — if partial mode off, allow all; else allow listed
// kinds (Assign also ok if Invoke listed); first success clears MAX sentinel.
func ExpandCheck(t StatementType) bool {
	if partialExpands == nil || !partialExpands[MaxStatementType] {
		// not in partial mode → all valid
		return true
	}
	rv := partialExpands[t]
	if t == StmtAssign {
		rv = rv || partialExpands[StmtInvoke]
	}
	if rv {
		// after first successful expand, disable further forcing
		partialExpands[MaxStatementType] = false
	}
	return rv
}

// ClearPartialExpander resets package state (tests / finalization).
func ClearPartialExpander() {
	partialExpands = nil
	partialExpandsBackup = nil
}

// InitPartialExpanderFromOptions wires CGOptions::partial_expand string.
func InitPartialExpanderFromOptions(opts Options) bool {
	if opts.PartialExpand == "" {
		ClearPartialExpander()
		return true
	}
	return InitPartialExpander(opts.PartialExpand)
}
