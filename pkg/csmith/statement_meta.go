// Upstream: Statement.cpp in_block, dominate, find_container_stm, is_1st_stm,
// is_jump_target_from_other_blocks, get_blk_depth, is_ptr_used.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// GetBlkDepth mirrors Statement::get_blk_depth.
// Statement.cpp:348–354 — count parent block chain.
func GetBlkDepth(parent *Block) int {
	depth := 0
	for b := parent; b != nil; b = b.Parent {
		depth++
	}
	return depth
}

// StmtInBlock mirrors Statement::in_block.
// Statement.cpp:380–389 — parent chain contains b.
func StmtInBlock(stParent, b *Block) bool {
	for tmp := stParent; tmp != nil; tmp = tmp.Parent {
		if tmp == b {
			return true
		}
	}
	return false
}

// GetBlocksStmt returns child blocks of a statement (Statement::get_blocks).
// Then/Else for if/for; nested only — not the parent block.
func GetBlocksStmt(st *Stmt) []*Block {
	if st == nil {
		return nil
	}
	var out []*Block
	if st.Then != nil {
		out = append(out, st.Then)
	}
	if st.Else != nil {
		out = append(out, st.Else)
	}
	return out
}

// FindTypedStmts mirrors Statement::find_typed_stmts.
// Statement.cpp:631–646 — collect statements whose Kind is in kinds (recursive).
// Returns count of matches appended to stms, or -1 on incomplete IR
// (nil Block hole — no invent partial typed-stmt list past hole; *stms cleared).
func FindTypedStmts(st *Stmt, stms *[]*Stmt, kinds []StatementType) int {
	if stms == nil {
		return -1
	}
	if st == nil {
		// incomplete Statement* — fail closed clear (no invent empty match success)
		*stms = nil
		return -1
	}
	for _, k := range kinds {
		if st.Kind == k {
			*stms = append(*stms, st)
			break
		}
	}
	for _, b := range GetBlocksStmt(st) {
		// Block* always live from get_blocks; nil hole fails closed
		if b == nil {
			*stms = nil
			return -1
		}
		for i := range b.Stmts {
			if n := FindTypedStmts(&b.Stmts[i], stms, kinds); n < 0 {
				return -1
			}
		}
	}
	return len(*stms)
}

// FindTypedStmtsInBlock walks a block's statements for typed collection.
// Returns -1 on incomplete IR (same as FindTypedStmts).
func FindTypedStmtsInBlock(b *Block, stms *[]*Stmt, kinds []StatementType) int {
	if stms == nil {
		return -1
	}
	if b == nil {
		*stms = nil
		return -1
	}
	for i := range b.Stmts {
		if n := FindTypedStmts(&b.Stmts[i], stms, kinds); n < 0 {
			return -1
		}
	}
	return len(*stms)
}

// Is1stStm mirrors Statement::is_1st_stm.
// Statement.cpp:649–651 — first statement of parent block.
func Is1stStm(st *Stmt, parent *Block) bool {
	if st == nil || parent == nil || len(parent.Stmts) == 0 {
		return false
	}
	first := &parent.Stmts[0]
	if first == st {
		return true
	}
	return first.StmID > 0 && first.StmID == st.StmID
}

// FindContainerStm mirrors Statement::find_container_stm for a nested block.
// Statement.cpp:414–430 — parent-block statement whose Then/Else is b.
func FindContainerStm(b *Block) *Stmt {
	if b == nil || b.Parent == nil {
		return nil
	}
	for i := range b.Parent.Stmts {
		s := &b.Parent.Stmts[i]
		if s.Then == b || s.Else == b {
			return s
		}
	}
	return nil
}

// Dominate mirrors Statement::dominate.
// Statement.cpp:393–410 — a dominates s (same block order or ancestor container).
// aParent / sParent are parent blocks of a and s (Stmt has no Parent field).
func Dominate(a *Stmt, aParent *Block, s *Stmt, sParent *Block) bool {
	if a == nil || s == nil {
		return false
	}
	// s is nested inside a (a's Then/Else is s's parent block)
	if sParent != nil && (a.Then == sParent || a.Else == sParent) {
		return true
	}
	// same parent: earlier stm_id dominates later
	if aParent == sParent {
		if aParent != nil && (a.StmID == 0 || s.StmID == 0) {
			ia, is := -1, -1
			for i := range aParent.Stmts {
				if &aParent.Stmts[i] == a || (a.StmID > 0 && aParent.Stmts[i].StmID == a.StmID) {
					ia = i
				}
				if &aParent.Stmts[i] == s || (s.StmID > 0 && aParent.Stmts[i].StmID == s.StmID) {
					is = i
				}
			}
			if ia >= 0 && is >= 0 {
				return ia <= is
			}
		}
		return a.StmID <= s.StmID
	}
	// walk container of s (if/for that owns sParent)
	if sParent != nil {
		container := FindContainerStm(sParent)
		if container != nil {
			return Dominate(a, aParent, container, sParent.Parent)
		}
	}
	return false
}

// IsJumpTargetFromOtherBlocks mirrors Statement::is_jump_target_from_other_blocks.
// Statement.cpp:653–663 — goto sources from a different parent block.
// destParent is this statement's parent; srcParentOf maps src StmID → parent block.
// When srcParentOf is nil, treat non-sibling StmIDs in destParent as other-block sources.
// Incomplete CFG (FindJumpSources nil) fails closed as jump-target (no invent none).
// Nil FactMgr fails closed as jump-target (no invent "not target" without CFG).
func IsJumpTargetFromOtherBlocks(destStmID int, destParent *Block, fm *FactMgr, srcParentOf map[int]*Block) bool {
	if destStmID <= 0 {
		return false
	}
	if fm == nil {
		return true
	}
	srcs := fm.FindJumpSources(destStmID)
	if srcs == nil {
		return true
	}
	for _, srcID := range srcs {
		if srcParentOf != nil {
			if srcParentOf[srcID] != destParent {
				return true
			}
			continue
		}
		// no parent map: sibling if source is a statement in destParent
		sibling := false
		if destParent != nil {
			for i := range destParent.Stmts {
				if destParent.Stmts[i].StmID == srcID {
					sibling = true
					break
				}
			}
		}
		if !sibling {
			return true
		}
	}
	return false
}

// IsPtrUsed mirrors Statement::is_ptr_used.
// Statement.cpp:355–359.
// Incomplete IR fails closed as true (no invent "no pointer used" past holes).
func IsPtrUsed(st *Stmt) bool {
	var ptrs []*Variable
	if !collectReferencedPtrsStmt(st, &ptrs) {
		return true
	}
	return len(ptrs) > 0
}

// ContainsStmtInBlock mirrors Block as Statement::contains_stmt for a block root.
// Statement.cpp:684–694 — s's parent chain includes block.
func ContainsStmtInBlock(b *Block, stParent *Block) bool {
	if b == nil {
		return false
	}
	return StmtInBlock(stParent, b)
}

// ContainsStmtTree mirrors Statement::contains_stmt for compound statements.
// Statement.cpp:684–705 — self or nested Then/Else trees by StmID.
func ContainsStmtTree(root, s *Stmt) bool {
	if root == nil || s == nil {
		return false
	}
	if root == s || (root.StmID > 0 && root.StmID == s.StmID) {
		return true
	}
	if root.Then != nil && (blockHasStmtIDDeep(root.Then, s.StmID) || root.Then == s.Then) {
		if s.StmID > 0 && blockHasStmtIDDeep(root.Then, s.StmID) {
			return true
		}
	}
	if root.Else != nil && s.StmID > 0 && blockHasStmtIDDeep(root.Else, s.StmID) {
		return true
	}
	return false
}

func blockHasStmtIDDeep(b *Block, id int) bool {
	if b == nil || id <= 0 {
		return false
	}
	if b.StmID == id {
		return true
	}
	for i := range b.Stmts {
		if b.Stmts[i].StmID == id {
			return true
		}
		if b.Stmts[i].Then != nil && blockHasStmtIDDeep(b.Stmts[i].Then, id) {
			return true
		}
		if b.Stmts[i].Else != nil && blockHasStmtIDDeep(b.Stmts[i].Else, id) {
			return true
		}
	}
	return false
}
