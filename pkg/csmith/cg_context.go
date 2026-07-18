// Upstream: CGContext.h / CGContext.cpp (empty context + effect_context accessors).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// CGContext is a minimal CGContext for paths that only need effect_context.
// Function/block/fact fields land with later ports.
type CGContext struct {
	effectContext Effect
	// CurrentFunc mirrors current_func_ (Function*).
	CurrentFunc *Function
	// BlkDepth mirrors blk_depth.
	BlkDepth int
	// Flags mirrors CGContext flags (IN_LOOP etc.).
	Flags uint
	// ExprDepth mirrors expr_depth.
	ExprDepth int
}

// EmptyCGContext mirrors CGContext::get_empty_context() (empty effect context).
func EmptyCGContext() CGContext {
	return CGContext{effectContext: EmptyEffect()}
}

// EffectContext mirrors CGContext::get_effect_context.
func (c CGContext) EffectContext() Effect {
	return c.effectContext
}

// WithEffectContext returns a context with the given effect_context.
func WithEffectContext(eff Effect) CGContext {
	return CGContext{effectContext: eff}
}

// WithFunc returns a context for generating inside f.
func WithFunc(f *Function, eff Effect) CGContext {
	return CGContext{effectContext: eff, CurrentFunc: f}
}

// InLoop is true when flags include IN_LOOP (2).
func (c CGContext) InLoop() bool { return c.Flags&2 != 0 }

