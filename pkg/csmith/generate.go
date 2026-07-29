// Upstream: AbsProgramGenerator / DefaultProgramGenerator (goGenerator entry).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
//
// Stub: intentionally not body-parity with golden csmith. The bodyparity
// harness (test/bodyparity) is the convergence loop — implement Library.Generate
// so pre-stats program bodies match upstream for the same drop-in Options + seed.
package csmith

import (
	"context"
	"strconv"
	"strings"
)

// Generator is the drop-in generation contract: Options → full C program text
// (upstream csmith stdout role). The in-process Go port (Library) and the
// bodyparity upstream-binary adapter both implement this so the harness
// compares two Generators with the same call shape and parent-owned context.
type Generator interface {
	Generate(ctx context.Context, opts Options) (string, error)
}

// Library is the in-process Go generator. Stub until body parity converges.
type Library struct{}

// Compile-time check: Library is a Generator.
var _ Generator = Library{}

// Generate is the preferred package entry (SPEC §2.2). Parent owns ctx
// (timeouts, cancel). Equivalent to Library{}.Generate.
func Generate(ctx context.Context, opts Options) (string, error) {
	return Library{}.Generate(ctx, opts)
}

// Generate emits a full C program (upstream stdout role).
// Stub: minimal skeleton so the package builds and the harness fails with a
// go-cmp body diff until a real generator is ported.
func (Library) Generate(ctx context.Context, opts Options) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if opts.PlatformInfoPath != "" {
		if resolved, err := opts.resolvePlatformInfo(); err == nil {
			opts = resolved
		}
	}
	opts = opts.normalizeUpstreamFlow()
	if err := opts.Validate(); err != nil {
		return "", err
	}

	// Stub program body: enough structure for programBody() in bodyparity to
	// extract a section, but never bit-identical to golden (drives convergence).
	var b strings.Builder
	b.WriteString("/*\n")
	b.WriteString(" * This is a RANDOMLY GENERATED PROGRAM.\n")
	b.WriteString(" *\n")
	b.WriteString(" * Generator: go-csmith (stub)\n")
	b.WriteString(" * Options:  ")
	b.WriteString(FormatOptionsShort(opts))
	b.WriteString("\n")
	b.WriteString(" * Seed:      ")
	b.WriteString(strconv.FormatUint(opts.Seed, 10))
	b.WriteString("\n */\n\n")
	b.WriteString("/* --- Struct/Union Declarations --- */\n")
	b.WriteString("/* STUB: implement type/struct emit vs upstream */\n\n")
	b.WriteString("static long __undefined;\n\n")
	b.WriteString("/* --- GLOBAL VARIABLES --- */\n\n")
	b.WriteString("/* --- FORWARD DECLARATIONS --- */\n")
	b.WriteString("int main (void);\n\n")
	b.WriteString("/* --- FUNCTIONS --- */\n")
	b.WriteString("/* ------------------------------------------ */\n")
	b.WriteString("int main (void)\n{\n")
	b.WriteString("    return 0;\n")
	b.WriteString("}\n\n")
	b.WriteString("/************************ statistics *************************\n")
	b.WriteString("*** stub — not parity ***\n")
	b.WriteString("**********************************************************/\n")
	return b.String(), nil
}
