// Upstream: CFGEdge.h / FactMgr::create_cfg_edge.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// CFGEdge mirrors CFGEdge — control-flow edge between statements/blocks.
// CFGEdge.h:43–55. Src/Dest stored by StmID (Go values are copied into Block.Stmts).
type CFGEdge struct {
	// SrcID is Statement::stm_id of the source.
	SrcID int
	// DestBlock is the destination block (loop head/end or target's block).
	DestBlock *Block
	// DestStmID is optional target statement id (goto); 0 if dest is block only.
	DestStmID int
	// PostDest mirrors post_dest.
	PostDest bool
	// BackLink mirrors back_link (continue → loop head; backward goto).
	BackLink bool
}

// ClosestLoopingBlock walks parents until looping is true.
// StatementBreak.cpp:71–75 / StatementContinue.cpp:73–76.
// Block always live at break/continue; sticky nil (no invent no-loop soft-skip past hole).
// Non-looping chain end (Parent nil) is complete miss (not incomplete IR).
func ClosestLoopingBlock(b *Block) *Block {
	if b == nil {
		SetError(ErrGeneric)
		return nil
	}
	for cur := b; cur != nil; cur = cur.Parent {
		if cur.Looping {
			return cur
		}
	}
	return nil
}

// CFGEdgesComplete reports every CFGEdge* is live (no nil holes).
// Note: CFGEdgesComplete(nil)==true (complete empty). Fail-closed incomplete
// wipes must use IncompleteCFGEdges() so len(nil)==0 cannot invent empty-complete
// edge-set success after scrub/analysis holes.
func CFGEdgesComplete(edges []*CFGEdge) bool {
	for _, e := range edges {
		if e == nil {
			return false
		}
	}
	return true
}

// IncompleteCFGEdges is the fail-closed incomplete CFG edge-list marker.
// CFGEdgesComplete returns false. Distinct from complete empty (nil or {}).
func IncompleteCFGEdges() []*CFGEdge {
	return []*CFGEdge{nil}
}

// BlocksComplete reports every Block* is live (no nil holes).
// Note: BlocksComplete(nil)==true (complete empty). Fail-closed incomplete
// wipes must use IncompleteBlocks() so bare nil cannot invent empty-complete
// Function.Blocks after scrub holes.
func BlocksComplete(blks []*Block) bool {
	for _, b := range blks {
		if b == nil {
			return false
		}
	}
	return true
}

// IncompleteBlocks is the fail-closed incomplete Block* list marker.
func IncompleteBlocks() []*Block {
	return []*Block{nil}
}
