// Upstream: PartialExpander.h / PartialExpander.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// PartialExpander gates which statement kinds may be generated when --partial-expand is set.
// When inactive (MAX key false), ExpandCheck always allows all kinds.

// InitPartialExpander mirrors PartialExpander::init_partial_expander.
// PartialExpander.cpp:59–67 — parse comma-separated kind names; enable MAX sentinel.
// Empty options fail closed false (C++ parse_options("") → set_expand("") false).
func InitPartialExpander(options string) bool {
	return InitPartialExpanderSess(testAmbientSession, options)
}

// InitPartialExpanderSess is InitPartialExpander on an explicit session bag.
func InitPartialExpanderSess(s *Session, options string) bool {
	s = sessOrAmbient(s)
	s.PartialExpands = initPartialMap(false)
	if !parsePartialOptionsSess(s, options, ',') {
		// leave PartialExpands half-init; callers treat false as fail
		return false
	}
	// MAX_STATEMENT_TYPE sentinel means "partial mode active"
	s.PartialExpands[MaxStatementType] = true
	s.PartialExpandsBackup = copyPartialMap(s.PartialExpands)
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
	return parsePartialOptionsSess(testAmbientSession, options, sep)
}

func parsePartialOptionsSess(s *Session, options string, sep byte) bool {
	// PartialExpander.cpp:71–86 — empty string yields one empty token → set_expand fails
	if options == "" {
		return false
	}
	parts := SplitString(options, sep)
	if len(parts) == 0 {
		// single token without separator (SplitString emptied)
		parts = []string{options}
	}
	for _, tok := range parts {
		if !setPartialExpandSess(s, tok) {
			return false
		}
	}
	return true
}

func setPartialExpand(tok string) bool {
	return setPartialExpandSess(testAmbientSession, tok)
}

func setPartialExpandSess(s *Session, tok string) bool {
	s = sessOrAmbient(s)
	switch tok {
	case "assignment":
		s.PartialExpands[StmtAssign] = true
	case "block":
		s.PartialExpands[StmtBlock] = true
	case "for":
		s.PartialExpands[StmtFor] = true
	case "if-else":
		s.PartialExpands[StmtIfElse] = true
	case "invoke":
		s.PartialExpands[StmtInvoke] = true
	case "return":
		s.PartialExpands[StmtReturn] = true
	case "all":
		s.PartialExpands = initPartialMap(true)
	default:
		return false
	}
	return true
}

// SetStmtExpand mirrors PartialExpander::set_stmt_expand.
func SetStmtExpand(t StatementType, value bool) {
	SetStmtExpandSess(testAmbientSession, t, value)
}

// SetStmtExpandSess is SetStmtExpand on an explicit session bag.
func SetStmtExpandSess(s *Session, t StatementType, value bool) {
	s = sessOrAmbient(s)
	if s.PartialExpands == nil {
		s.PartialExpands = initPartialMap(false)
	}
	s.PartialExpands[t] = value
}

// RestorePartialExpanderInitValuesSess restores PartialExpander init values on bag s.
// Mirrors restore_init_values (PartialExpander.cpp:122–125).
// Non-Sess RestorePartialExpanderInitValues deleted — pass bag explicitly.
func RestorePartialExpanderInitValuesSess(s *Session) {
	s = sessOrAmbient(s)
	if s.PartialExpandsBackup != nil {
		s.PartialExpands = copyPartialMap(s.PartialExpandsBackup)
	}
}

// DirectExpandCheckSess is DirectExpandCheck on an explicit session bag.
func DirectExpandCheckSess(s *Session, t StatementType) bool {
	s = sessOrAmbient(s)
	if s.PartialExpands == nil {
		return false
	}
	return s.PartialExpands[t]
}

// ExpandCheck mirrors PartialExpander::expand_check.
// PartialExpander.cpp:132–151 — if partial mode off, allow all; else allow listed
// kinds (Assign also ok if Invoke listed); first success clears MAX sentinel.
func ExpandCheck(t StatementType) bool {
	return ExpandCheckSess(testAmbientSession, t)
}

// ExpandCheckSess is ExpandCheck on an explicit session bag.
func ExpandCheckSess(s *Session, t StatementType) bool {
	s = sessOrAmbient(s)
	if s.PartialExpands == nil || !s.PartialExpands[MaxStatementType] {
		// not in partial mode → all valid
		return true
	}
	// PartialExpander.cpp:137 — assert(expands_.find(t) != end)
	// map only holds Assign/Block/For/IfElse/Invoke/Return/MAX; other kinds fail closed
	// (filter rejects), not soft invent allow-all for unregistered types
	rv := s.PartialExpands[t]
	if t == StmtAssign {
		// Assign also ok when Invoke is listed (PartialExpander.cpp:143–145)
		rv = rv || s.PartialExpands[StmtInvoke]
	}
	if rv {
		// after first successful expand, disable further forcing
		s.PartialExpands[MaxStatementType] = false
	}
	return rv
}

// ClearPartialExpanderSess clears partial-expand state on an explicit session bag.
func ClearPartialExpanderSess(s *Session) {
	s = sessOrAmbient(s)
	s.PartialExpands = nil
	s.PartialExpandsBackup = nil
}

// InitPartialExpanderFromOptionsSess wires partial-expand on an explicit session bag.
func InitPartialExpanderFromOptionsSess(s *Session, opts Options) bool {
	if opts.PartialExpand == "" {
		ClearPartialExpanderSess(s)
		return true
	}
	return InitPartialExpanderSess(s, opts.PartialExpand)
}
