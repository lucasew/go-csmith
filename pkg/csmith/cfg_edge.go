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
func ClosestLoopingBlock(b *Block) *Block {
	for b != nil && !b.Looping {
		b = b.Parent
	}
	return b
}
