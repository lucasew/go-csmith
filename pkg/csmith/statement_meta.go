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
// Kind-gated like C++ overrides — assign/break/goto push nothing.
// StatementIf always pushes if_true and if_false (live Block refs in C++);
// nil arms are incomplete IR so walkers fail closed (no invent soft-skip missing arm).
// StatementFor always pushes body; ArrayOp/Block push Then when non-nil (C++ if(body)).
func GetBlocksStmt(st *Stmt) []*Block {
	return GetBlocksStmtSess(testAmbientSession, st)
}

func GetBlocksStmtSess(s *Session, st *Stmt) []*Block {
	// Statement always live; sticky incomplete IncompleteBlocks (no invent empty-complete)
	if st == nil {
		sessNoteError(s, ErrGeneric)
		return IncompleteBlocks()
	}
	switch st.Kind {
	case StmtIfElse:
		// StatementIf.h — blks.push_back(&if_true); blks.push_back(&if_false);
		// both arms always live in C++; nil arm sticky IncompleteBlocks
		// (no invent []*Block{Then,nil} soft list for walkers to soft-continue past)
		if st.Then == nil || st.Else == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteBlocks()
		}
		return []*Block{st.Then, st.Else}
	case StmtFor:
		// StatementFor.h — blks.push_back(&body); body always live
		// sticky IncompleteBlocks (no invent []*Block{nil} soft for-body hole)
		if st.Then == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteBlocks()
		}
		return []*Block{st.Then}
	case StmtArrayOp:
		// StatementArrayOp.h:69–71 — if (body) blks.push_back(body).
		// make_random_array_init uses Expression ctor (body=0, init_value=e);
		// get_blocks is empty. Go nests RHS under Then for Output only — must not
		// invent get_blocks walk of that Then (extra Lhs/expr ptrs → ReferencedPtrs
		// / NeedsRevisit drift vs UP, seed-2 func_49 revisit soft-fail).
		if st.Then == nil {
			return nil
		}
		inner := findArrayOpInnermostSess(s, st)
		if inner != nil && isArrayInitBodySess(s, inner.Then) {
			return nil
		}
		// true body path (Block ctor) — rare; keep Then when not array-init shape
		return []*Block{st.Then}
	case StmtBlock:
		// nested Block::make_random body
		if st.Then != nil {
			return []*Block{st.Then}
		}
		return nil
	default:
		// assign/invoke/return/break/continue/goto — empty get_blocks
		return nil
	}
}

// StmtsComplete reports every Stmt* is live (no nil holes).
// Note: StmtsComplete(nil)==true (complete empty). Fail-closed incomplete
// wipes must use IncompleteStmtsSlice() so len(nil)==0 cannot invent
// empty-complete typed-stmt list success.}

func StmtsComplete(stms []*Stmt) bool {
	for _, s := range stms {
		if s == nil {
			return false
		}
	}
	return true
}

// IncompleteStmtsSlice is the fail-closed incomplete Stmt* list marker.
// StmtsComplete returns false. Distinct from complete empty (nil or {}).
func IncompleteStmtsSlice() []*Stmt {
	return []*Stmt{nil}
}

// FindTypedStmts mirrors Statement::find_typed_stmts.
// Statement.cpp:631–646 — collect statements whose Kind is in kinds (recursive).
// Returns count of matches appended to stms, or -1 on incomplete IR sticky
// (nil Block hole — *stms IncompleteStmtsSlice; no invent empty match / soft re-pick).
func FindTypedStmts(st *Stmt, stms *[]*Stmt, kinds []StatementType) int {
	return FindTypedStmtsSess(testAmbientSession, st, stms, kinds)
}

func FindTypedStmtsSess(s *Session, st *Stmt, stms *[]*Stmt, kinds []StatementType) int {
	if stms == nil {
		return -1
	}
	if st == nil {
		// incomplete Statement* — fail closed sticky hole marker
		*stms = IncompleteStmtsSlice()
		sessNoteError(s, ErrGeneric)
		return -1
	}
	for _, k := range kinds {
		if st.Kind == k {
			*stms = append(*stms, st)
			break
		}
	}
	blks := GetBlocksStmt(st)
	// residual ERROR sticky — no invent soft-walk past GetBlocksStmt residual
	if sessHasError(s) {
		*stms = IncompleteStmtsSlice()
		return -1
	}
	for _, b := range blks {
		// Block* always live from get_blocks; nil hole fails closed sticky
		if b == nil {
			*stms = IncompleteStmtsSlice()
			sessNoteError(s, ErrGeneric)
			return -1
		}
		for i := range b.Stmts {
			if n := FindTypedStmtsSess(s, &b.Stmts[i], stms, kinds); n < 0 {
				// residual ERROR sticky — no invent soft-continue walk past child residual
				if sessHasError(s) {
					if StmtsComplete(*stms) {
						*stms = IncompleteStmtsSlice()
					}
					return -1
				}
				// child already set IncompleteStmtsSlice sticky when it failed closed
				if StmtsComplete(*stms) {
					*stms = IncompleteStmtsSlice()
					sessNoteError(s, ErrGeneric)
				}
				return -1
			}
			// residual ERROR sticky — no invent soft-continue later stmts past child residual true
			if sessHasError(s) {
				*stms = IncompleteStmtsSlice()
				return -1
			}
		}
	}
	return len(*stms)
}

// FindTypedStmtsInBlock walks a block's statements for typed collection.
// Returns -1 on incomplete IR sticky (same as FindTypedStmts).}

func FindTypedStmtsInBlock(b *Block, stms *[]*Stmt, kinds []StatementType) int {
	return FindTypedStmtsInBlockSess(testAmbientSession, b, stms, kinds)
}

func FindTypedStmtsInBlockSess(s *Session, b *Block, stms *[]*Stmt, kinds []StatementType) int {
	if stms == nil {
		return -1
	}
	if b == nil {
		*stms = IncompleteStmtsSlice()
		sessNoteError(s, ErrGeneric)
		return -1
	}
	for i := range b.Stmts {
		if n := FindTypedStmtsSess(s, &b.Stmts[i], stms, kinds); n < 0 {
			if StmtsComplete(*stms) {
				*stms = IncompleteStmtsSlice()
			}
			return -1
		}
	}
	return len(*stms)
}

// Is1stStm mirrors Statement::is_1st_stm.
// Statement.cpp:649–651 — first statement of parent block.
// Incomplete Statement/parent sticky false (no invent is-first / soft re-pick past holes).}

func Is1stStm(st *Stmt, parent *Block) bool {
	return Is1stStmSess(testAmbientSession, st, parent)
}

// Is1stStmSess is Is1stStm with explicit session residual sticky.
func Is1stStmSess(s *Session, st *Stmt, parent *Block) bool {
	// Statement + parent always live; sticky incomplete no invent is-first soft-skip
	if st == nil || parent == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if len(parent.Stmts) == 0 {
		return false
	}
	first := &parent.Stmts[0]
	if first == st {
		return true
	}
	return !StmIDUnset(first.StmID) && first.StmID == st.StmID
}

// FindContainerStm mirrors Statement::find_container_stm for a nested block.
// Statement.cpp:414–430 — parent-block statement whose get_blocks contains b.
// Kind-gated get_blocks only (no invent container via stray Then on assign).
// Block always live; sticky nil (no invent soft-skip missing container past hole).
// Missing parent is complete nil (root block — not incomplete).
// Incomplete parent get_blocks hole sticky nil (no invent soft-skip missing container).
func FindContainerStm(b *Block) *Stmt {
	return FindContainerStmSess(testAmbientSession, b)
}

// FindContainerStmSess is FindContainerStm with explicit session residual sticky.
func FindContainerStmSess(s *Session, b *Block) *Stmt {
	if b == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// root is complete nil (not incomplete)
	if b.Parent == nil {
		return nil
	}
	for i := range b.Parent.Stmts {
		st := &b.Parent.Stmts[i]
		blks := GetBlocksStmtSess(s, st)
		// incomplete get_blocks (if arms / for body / block holes) sticky no invent miss
		// scan all arms first — no invent match Then then soft-skip nil Else
		incomplete := false
		matched := false
		for _, nb := range blks {
			if nb == nil {
				incomplete = true
				continue
			}
			if nb == b {
				matched = true
			}
		}
		if incomplete {
			// get_blocks arms always live when Kind exposes them; nil hole sticky
			sessNoteError(s, ErrGeneric)
			return nil
		}
		if matched {
			return st
		}
	}
	return nil
}

// Dominate mirrors Statement::dominate.
// Statement.cpp:393–410 — a dominates s (same block order or ancestor container).
// aParent / sParent are parent blocks of a and s (Stmt has no Parent field).
// Statement::stm_id is always live after create; StmID 0 is incomplete IR.
// Same-parent incomplete IDs resolve by parent membership when possible; unresolved
// membership fails closed sticky false (no invent dominate true via 0<=0 / soft re-pick).
// Nested-in-a uses get_blocks only (no invent dominate via stray Then on assign).
// Incomplete get_blocks arms sticky false (no invent match Then then soft-skip nil Else).
func Dominate(a *Stmt, aParent *Block, s *Stmt, sParent *Block) bool {
	return DominateSess(testAmbientSession, a, aParent, s, sParent)
}

// DominateSess is Dominate with sticky on run bag (sess; stmt param remains s).
func DominateSess(sess *Session, a *Stmt, aParent *Block, s *Stmt, sParent *Block) bool {
	// both Statement* always live; sticky incomplete no invent not-dominate soft-skip
	if a == nil || s == nil {
		sessNoteError(sess, ErrGeneric)
		return false
	}
	// s is nested inside a (get_blocks of a includes s's parent block)
	if sParent != nil {
		blks := GetBlocksStmtSess(sess, a)
		incomplete := false
		matched := false
		for _, nb := range blks {
			if nb == nil {
				// incomplete arm sticky — no invent dominate via Then past nil Else
				incomplete = true
				continue
			}
			if nb == sParent {
				matched = true
			}
		}
		if incomplete {
			sessNoteError(sess, ErrGeneric)
			return false
		}
		if matched {
			return true
		}
	}
	// same parent: earlier stm_id dominates later (Statement.cpp:399–401)
	if aParent == sParent {
		if aParent != nil && (StmIDUnset(a.StmID) || StmIDUnset(s.StmID)) {
			ia, is := -1, -1
			for i := range aParent.Stmts {
				if &aParent.Stmts[i] == a || (!StmIDUnset(a.StmID) && aParent.Stmts[i].StmID == a.StmID) {
					ia = i
				}
				if &aParent.Stmts[i] == s || (!StmIDUnset(s.StmID) && aParent.Stmts[i].StmID == s.StmID) {
					is = i
				}
			}
			if ia >= 0 && is >= 0 {
				return ia <= is
			}
			// incomplete StmID and not both in parent — sticky fail closed
			sessNoteError(sess, ErrGeneric)
			return false
		}
		return a.StmID <= s.StmID
	}
	// walk container of s (if/for that owns sParent)
	// FindContainerStm stickies incomplete arms; residual ERROR fails closed
	if sParent != nil {
		container := FindContainerStmSess(sess, sParent)
		if container != nil {
			return DominateSess(sess, a, aParent, container, sParent.Parent)
		}
		if sessHasError(sess) {
			return false
		}
	}
	return false
}

func IsJumpTargetFromOtherBlocks(destStmID int, destParent *Block, fm *FactMgr, srcParentOf map[int]*Block) bool {
	if StmIDUnset(destStmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return true
	}
	// FactMgr always live for CFG jump sources; sticky fail closed jump-target
	// (no invent "not target" without CFG / soft re-pick past hole)
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return true
	}
	srcs := fm.FindJumpSources(destStmID)
	// incomplete CFG (nil sources) sticky fail closed as jump-target
	if srcs == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
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
// Incomplete IR fails closed sticky as true (no invent "no pointer used" / soft re-pick past holes).
func IsPtrUsed(st *Stmt) bool {
	return IsPtrUsedSess(testAmbientSession, st)
}

// IsPtrUsedSess is IsPtrUsed with explicit session residual sticky.
func IsPtrUsedSess(s *Session, st *Stmt) bool {
	var ptrs []*Variable
	CollectReferencedPtrsStmtSess(s, st, &ptrs)
	if !VariablesComplete(ptrs) {
		// CollectReferencedPtrsStmt already SetError sticky
		return true
	}
	return len(ptrs) > 0
}

// ContainsStmtInBlock mirrors Block as Statement::contains_stmt for a block root.
// Statement.cpp:684–694 — s's parent chain includes block.
// Incomplete block root sticky false (no invent not-contain / soft re-pick past hole).
func ContainsStmtInBlock(b *Block, stParent *Block) bool {
	return ContainsStmtInBlockSess(testAmbientSession, b, stParent)
}

// ContainsStmtInBlockSess is ContainsStmtInBlock with explicit session residual sticky.
func ContainsStmtInBlockSess(s *Session, b *Block, stParent *Block) bool {
	// Block root always live; sticky incomplete no invent not-contain
	if b == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return StmtInBlock(stParent, b)
}

// ContainsStmtTree mirrors Statement::contains_stmt for compound statements.
// Statement.cpp:684–705 — self or nested get_blocks trees by StmID.
// Incomplete Block* hole fails closed sticky false (no invent membership / soft re-pick).
// Walks only get_blocks kinds (no invent search via stray Then on assign).
func ContainsStmtTree(root, s *Stmt) bool {
	return ContainsStmtTreeSess(testAmbientSession, root, s)
}

// ContainsStmtTreeSess is ContainsStmtTree with explicit session residual sticky.
func ContainsStmtTreeSess(sess *Session, root, s *Stmt) bool {
	// both Statement* always live; sticky incomplete no invent not-contain soft-skip
	if root == nil || s == nil {
		sessNoteError(sess, ErrGeneric)
		return false
	}
	if root == s || (!StmIDUnset(root.StmID) && root.StmID == s.StmID) {
		return true
	}
	if StmIDUnset(s.StmID) {
		return false
	}
	blks := GetBlocksStmtSess(sess, root)
	// pre-validate complete get_blocks (if always has both arms)
	// nil hole sticky before invent membership from a partial arm scan
	for _, b := range blks {
		if b == nil {
			sessNoteError(sess, ErrGeneric)
			return false
		}
	}
	for _, b := range blks {
		if blockHasStmtIDDeepSess(sess, b, s.StmID) {
			return true
		}
	}
	return false
}

// blockHasStmtIDDeep walks get_blocks for stm_id under b.
// Block + live StmID always required; sticky false (no invent not-found soft-skip past hole).
// Incomplete arm sticky false (no invent membership via partial arm scan before hole).
func blockHasStmtIDDeep(b *Block, id int) bool {
	return blockHasStmtIDDeepSess(testAmbientSession, b, id)
}

// blockHasStmtIDDeepSess is blockHasStmtIDDeep with explicit session residual sticky.
func blockHasStmtIDDeepSess(s *Session, b *Block, id int) bool {
	if b == nil || id <= 0 {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if b.StmID == id {
		return true
	}
	for i := range b.Stmts {
		if b.Stmts[i].StmID == id {
			return true
		}
		// pre-validate complete get_blocks before invent match past incomplete arm
		blks := GetBlocksStmtSess(s, &b.Stmts[i])
		for _, nb := range blks {
			if nb == nil {
				sessNoteError(s, ErrGeneric)
				return false
			}
		}
		for _, nb := range blks {
			if blockHasStmtIDDeepSess(s, nb, id) {
				return true
			}
		}
	}
	return false
}
