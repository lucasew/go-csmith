// Upstream: OutputMgr.h / OutputMgr.cpp / DefaultOutputMgr.cpp
// (monitored_funcs / curr_func / hash helpers / split-file emit paths).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Process OutputMgr statics (OutputMgr.cpp:72–86).
var (
	// monitoredFuncs mirrors OutputMgr::monitored_funcs_.
	monitoredFuncs []string
	// currFunc mirrors OutputMgr::curr_func_.
	currFunc string
)

// HashFuncName mirrors OutputMgr::hash_func_name.
// OutputMgr.cpp:51.
const HashFuncName = "csmith_compute_hash"

// StepHashFuncName mirrors OutputMgr::step_hash_func_name.
// OutputMgr.cpp:53.
const StepHashFuncName = "step_hash"

// DefaultOutputMgr split-file constants (DefaultOutputMgr.cpp:48–50).
const (
	// SplitFilenamePrefix is filename_prefix ("rnd_output").
	SplitFilenamePrefix = "rnd_output"
	// SplitGlobalHeader is global_header basename ("rnd_globals").
	SplitGlobalHeader = "rnd_globals"
)

// ParseStringOptions mirrors CGOptions::parse_string_options.
// CGOptions.cpp:562–577 — split on comma; empty input no-op.
// Does not trim spaces (upstream substr without strip).
func ParseStringOptions(vname string) []string {
	if vname == "" {
		return nil
	}
	var v []string
	pos1 := 0
	for {
		pos2 := -1
		for i := pos1; i < len(vname); i++ {
			if vname[i] == ',' {
				pos2 = i
				break
			}
		}
		if pos2 >= 0 {
			v = append(v, vname[pos1:pos2])
			pos1 = pos2 + 1
		} else {
			v = append(v, vname[pos1:])
			break
		}
	}
	return v
}

// SetMonitoredFuncs mirrors CGOptions::monitored_funcs.
// CGOptions.cpp:558–560 — parse into OutputMgr::monitored_funcs_.
func SetMonitoredFuncs(fnames string) {
	monitoredFuncs = ParseStringOptions(fnames)
}

// MonitoredFuncs returns a copy of OutputMgr::monitored_funcs_.
func MonitoredFuncs() []string {
	if len(monitoredFuncs) == 0 {
		return nil
	}
	return append([]string(nil), monitoredFuncs...)
}

// ClearMonitoredFuncs resets process monitored list (tests / finalization).
func ClearMonitoredFuncs() {
	monitoredFuncs = nil
	currFunc = ""
}

// SetCurrFunc mirrors OutputMgr::set_curr_func.
// OutputMgr.cpp:77–79.
func SetCurrFunc(fname string) {
	currFunc = fname
}

// CurrFunc mirrors OutputMgr::curr_func_.
func CurrFunc() string { return currFunc }

// IsMonitoredFunc mirrors OutputMgr::is_monitored_func.
// OutputMgr.cpp:81–86 — empty list → all monitored; else curr must be in list.
func IsMonitoredFunc() bool {
	if len(monitoredFuncs) == 0 {
		return true
	}
	for _, n := range monitoredFuncs {
		if n == currFunc {
			return true
		}
	}
	return false
}

// ReallyOutputLn mirrors OutputMgr::really_outputln.
// OutputMgr.cpp:352 — always emit newline (ignores compact/quiet).
func ReallyOutputLn() string { return "\n" }

// OutputLn mirrors OutputMgr::outputln / DefaultOutputMgr::outputln.
// OutputMgr.h default: out << endl. Compact DFS path is a separate gate.
func OutputLn() string { return "\n" }

// OutputHashFuncDecl mirrors OutputMgr::OutputHashFuncDecl.
// OutputMgr.cpp:200–203.
func OutputHashFuncDecl() string {
	return "void " + HashFuncName + "(void);\n\n"
}

// OutputStepHashFuncDecl mirrors OutputMgr::OutputStepHashFuncDecl.
// OutputMgr.cpp:205–208.
func OutputStepHashFuncDecl() string {
	return "void " + StepHashFuncName + "(int stmt_id);\n\n"
}

// OutputHashFuncInvocation mirrors OutputMgr::OutputHashFuncInvocation.
// OutputMgr.cpp:156–159 — tab(indent) + hash_func_name + "();\n".
func OutputHashFuncInvocation(indent int) string {
	return OutputTab(indent) + HashFuncName + "();\n"
}

// OutputStepHashFuncInvocation mirrors OutputMgr::OutputStepHashFuncInvocation.
// OutputMgr.cpp:161–167 — only when is_monitored_func; else soft empty.
func OutputStepHashFuncInvocation(indent, stmtID int) string {
	if !IsMonitoredFunc() {
		return ""
	}
	// incomplete stmt id sticky (no invent step_hash(0) shell)
	if stmtID <= 0 {
		SetError(ErrGeneric)
		return ""
	}
	return OutputTab(indent) + StepHashFuncName + "(" + Int2Str(stmtID) + ");\n"
}

// IsSplit mirrors DefaultOutputMgr::is_split.
// DefaultOutputMgr.cpp:207 — max_split_files() > 0.
func IsSplit(opts Options) bool { return opts.MaxSplitFiles > 0 }

// CreateOutputDir mirrors DefaultOutputMgr::create_output_dir.
// DefaultOutputMgr.cpp:99–101 → platform create_dir.
// Empty dir sticky false (no invent cwd as success).
func CreateOutputDir(dir string) bool {
	if dir == "" {
		SetError(ErrGeneric)
		return false
	}
	return CreateDir(dir)
}

// SplitOutputFilePath mirrors DefaultOutputMgr::open_one_output_file path build.
// DefaultOutputMgr.cpp:79–85 — split_files_dir/rnd_output{N}.c (dir_sep = OS separator).
// Empty split dir sticky "" (no invent relative bare name).
func SplitOutputFilePath(opts Options, num int) string {
	dir := strings.TrimSpace(opts.SplitFilesDir)
	if dir == "" {
		SetError(ErrGeneric)
		return ""
	}
	// incomplete negative index sticky (no invent rnd_output-1.c)
	if num < 0 {
		SetError(ErrGeneric)
		return ""
	}
	name := fmt.Sprintf("%s%d.c", SplitFilenamePrefix, num)
	return filepath.Join(dir, name)
}

// SplitGlobalsHeaderPath is DefaultOutputMgr::OutputGlobals header path.
// DefaultOutputMgr.cpp:104–105 — split_files_dir/rnd_globals.h.
func SplitGlobalsHeaderPath(opts Options) string {
	dir := strings.TrimSpace(opts.SplitFilesDir)
	if dir == "" {
		SetError(ErrGeneric)
		return ""
	}
	return filepath.Join(dir, SplitGlobalHeader+".h")
}

// SplitGlobalsHeaderBody mirrors DefaultOutputMgr::OutputGlobals skeleton.
// DefaultOutputMgr.cpp:108–116 — include guard + safe_math + extern decls + structs.
// decls is OutputGlobalVariablesDecls body; structDecls is OutputStructUnionDeclarations.
// Incomplete empty decls+structs soft returns guard-only shell still valid C++ shape.
func SplitGlobalsHeaderBody(decls, structDecls string) string {
	var b strings.Builder
	b.WriteString("#ifndef RND_GLOBALS_H\n")
	b.WriteString("#define RND_GLOBALS_H\n")
	b.WriteString("#include \"safe_math.h\"\n")
	b.WriteString(decls)
	b.WriteString(structDecls)
	b.WriteString("#endif\n")
	return b.String()
}

// SplitSecondaryHeaderPreamble mirrors DefaultOutputMgr::OutputAllHeaders secondary file.
// DefaultOutputMgr.cpp:121–129 — stdint / optional assert / limits + rnd_globals.h.
func SplitSecondaryHeaderPreamble(paranoid bool) string {
	var b strings.Builder
	b.WriteString("#include <stdint.h>\n")
	if paranoid {
		b.WriteString("#include <assert.h>\n")
	}
	b.WriteString("#include <limits.h>\n")
	b.WriteString("#include \"" + SplitGlobalHeader + ".h\"\n\n")
	return b.String()
}

// SplitPrimaryHeaderInclude mirrors DefaultOutputMgr::OutputAllHeaders outs[0].
// DefaultOutputMgr.cpp:132.
func SplitPrimaryHeaderInclude() string {
	return "#include \"" + SplitGlobalHeader + ".h\"\n"
}

// CompactOutputLn mirrors DFSOutputMgr::outputln when compact_output.
// DFSOutputMgr.cpp:94–97 — compact skips newlines.
func CompactOutputLn(compact bool) string {
	if compact {
		return ""
	}
	return "\n"
}

// CompactOutputCommentLine mirrors DFSOutputMgr::output_comment_line.
// DFSOutputMgr.cpp:99–103 — compact skips comments entirely.
func CompactOutputCommentLine(comment string, compact, quiet, concise bool) string {
	if compact {
		return ""
	}
	return OutputCommentLine(comment, quiet, concise)
}

// CompactOutputTab mirrors DFSOutputMgr::output_tab.
// DFSOutputMgr.cpp:105–108 — compact skips indent.
func CompactOutputTab(indent int, compact bool) string {
	if compact {
		return ""
	}
	return OutputTab(indent)
}
