// Upstream: OutputMgr.h / OutputMgr.cpp / DefaultOutputMgr.cpp
// (monitored_funcs / curr_func / hash helpers / split-file emit paths).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"path/filepath"
	"strings"
)

// OutputMgr statics live on Session (MonitoredFuncs, CurrFunc).

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
	currentSession().MonitoredFuncs = ParseStringOptions(fnames)
}

// MonitoredFuncs returns a copy of OutputMgr::monitored_funcs_.
func MonitoredFuncs() []string {
	if len(currentSession().MonitoredFuncs) == 0 {
		return nil
	}
	return append([]string(nil), currentSession().MonitoredFuncs...)
}

// ClearMonitoredFuncs resets process monitored list (tests / finalization).
func ClearMonitoredFuncs() {
	currentSession().MonitoredFuncs = nil
	currentSession().CurrFunc = ""
}

// SetCurrFunc mirrors OutputMgr::set_curr_func.
// OutputMgr.cpp:77–79.
func SetCurrFunc(fname string) {
	currentSession().CurrFunc = fname
}

// CurrFunc mirrors OutputMgr::curr_func_.
func CurrFunc() string { return currentSession().CurrFunc }

// IsMonitoredFunc mirrors OutputMgr::is_monitored_func.
// OutputMgr.cpp:81–86 — empty list → all monitored; else curr must be in list.
func IsMonitoredFunc() bool {
	if len(currentSession().MonitoredFuncs) == 0 {
		return true
	}
	for _, n := range currentSession().MonitoredFuncs {
		if n == currentSession().CurrFunc {
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
		sessNoteError(nil, ErrGeneric)
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
		sessNoteError(nil, ErrGeneric)
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
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// incomplete negative index sticky (no invent rnd_output-1.c)
	if num < 0 {
		sessNoteError(nil, ErrGeneric)
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
		sessNoteError(nil, ErrGeneric)
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

// --- DefaultOutputMgr process singleton (split-file emit) ---

// DefaultStructOutputName mirrors DFSOutputMgr DEFAULT_STRUCT_OUTPUT.
// DFSOutputMgr.h:36.
const DefaultStructOutputName = "csmith_structs.h"

// OutputMgrKind selects Default vs DFS emit orchestration.
type OutputMgrKind int

const (
	// OutputMgrKindDefault is DefaultOutputMgr.
	OutputMgrKindDefault OutputMgrKind = iota
	// OutputMgrKindDFS is DFSOutputMgr.
	OutputMgrKindDFS
)

// currentSession().OutputMgrKind / currentSession().StructOutput mirror CreateInstance selection state.

// CreateDefaultOutputMgr mirrors DefaultOutputMgr::CreateInstance + init.
// DefaultOutputMgr.cpp:62–97 — record ofile path; if max_split_files>0 build split paths.
// Does not open OS files (library returns paths/content); incomplete split dir sticky.
func CreateDefaultOutputMgr(opts Options) bool {
	return CreateDefaultOutputMgrSess(nil, opts)
}

// CreateDefaultOutputMgrSess is CreateDefaultOutputMgr on an explicit session bag.
func CreateDefaultOutputMgrSess(s *Session, opts Options) bool {
	s = sessOrAmbient(s)
	s.OutputMgrKind = OutputMgrKindDefault
	s.OutputFile = strings.TrimSpace(opts.OutputPath)
	s.SplitPaths = nil
	if !IsSplit(opts) {
		return true
	}
	// init: open_one_output_file path for each max_split_files
	n := opts.MaxSplitFiles
	if n <= 0 {
		return true
	}
	paths := make([]string, 0, n)
	for i := 0; i < n; i++ {
		p := SplitOutputFilePath(opts, i)
		if p == "" || sessHasError(s) {
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			s.SplitPaths = nil
			return false
		}
		paths = append(paths, p)
	}
	s.SplitPaths = paths
	return true
}

// CreateDFSOutputMgr mirrors DFSOutputMgr::CreateInstance.
// DFSOutputMgr.cpp:49–61 — struct_output_ default or CGOptions::struct_output.
func CreateDFSOutputMgr(opts Options) {
	CreateDFSOutputMgrSess(nil, opts)
}

// CreateDFSOutputMgrSess is CreateDFSOutputMgr on an explicit session bag.
func CreateDFSOutputMgrSess(s *Session, opts Options) {
	s = sessOrAmbient(s)
	s.OutputMgrKind = OutputMgrKindDFS
	s.SplitPaths = nil
	s.OutputFile = ""
	so := strings.TrimSpace(opts.StructOutput)
	if so == "" {
		s.StructOutput = DefaultStructOutputName
	} else {
		s.StructOutput = so
	}
}

// ClearOutputMgr resets process OutputMgr singleton state (finalization / tests).
func ClearOutputMgr() {
	ClearOutputMgrSess(nil)
}

// ClearOutputMgrSess clears OutputMgr selection state on an explicit session bag.
func ClearOutputMgrSess(s *Session) {
	s = sessOrAmbient(s)
	s.OutputMgrKind = OutputMgrKindDefault
	s.StructOutput = ""
	s.SplitPaths = nil
	s.OutputFile = ""
}

// ProcessOutputMgrKind returns active OutputMgr kind.
func ProcessOutputMgrKind() OutputMgrKind { return currentSession().OutputMgrKind }

// ProcessStructOutput returns DFSOutputMgr::struct_output_.
func ProcessStructOutput() string { return currentSession().StructOutput }

// ProcessSplitPaths returns a copy of DefaultOutputMgr split file paths.
func ProcessSplitPaths() []string {
	if len(currentSession().SplitPaths) == 0 {
		return nil
	}
	return append([]string(nil), currentSession().SplitPaths...)
}

// ProcessOutputFile returns DefaultOutputMgr ofile path (empty → stdout).
func ProcessOutputFile() string { return currentSession().OutputFile }

// GetMainOutPath mirrors DefaultOutputMgr::get_main_out target name.
// DefaultOutputMgr.cpp:197–205 — split → outs[0]; ofile_; else "" (stdout).
// Empty string means stdout (library has no ostream).
func GetMainOutPath(opts Options) string {
	if IsSplit(opts) {
		if len(currentSession().SplitPaths) == 0 {
			// not initialized sticky
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		return currentSession().SplitPaths[0]
	}
	return currentSession().OutputFile
}

// PureRndUptoIndex mirrors pure_rnd_upto for split file pick.
// DefaultOutputMgr.cpp:148 / 159 — index = pure_rnd_upto(size).
// nFiles==0 sticky -1 (no invent index 0 into empty outs).
func PureRndUptoIndex(nFiles int) int {
	if nFiles <= 0 {
		sessNoteError(nil, ErrGeneric)
		return -1
	}
	return int(PureRndUpto(uint32(nFiles), nil))
}

// RandomOutputVarDefs mirrors DefaultOutputMgr::RandomOutputVarDefs pure assignment.
// DefaultOutputMgr.cpp:144–151 — each global → pure_rnd_upto(nFiles) file bucket.
// Returns nFiles content strings (defs only, no headers). Incomplete globals sticky nil.
func RandomOutputVarDefs(globals []*Variable, nFiles int, forceStatic bool) []string {
	if nFiles <= 0 {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	if !VariablesComplete(globals) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	out := make([]string, nFiles)
	for _, v := range globals {
		idx := PureRndUptoIndex(nFiles)
		if idx < 0 || sessHasError(nil) {
			return nil
		}
		var def string
		if v.IsArray && v.AsArray != nil {
			def = v.AsArray.OutputDef()
		} else if v.IsArray {
			sessNoteError(nil, ErrGeneric)
			return nil
		} else {
			def = v.OutputDef(forceStatic)
		}
		if sessHasError(nil) || def == "" {
			if !sessHasError(nil) {
				sessNoteError(nil, ErrGeneric)
			}
			return nil
		}
		if !strings.HasSuffix(def, "\n") {
			def += "\n"
		}
		out[idx] += def
	}
	return out
}

// RandomOutputFuncDefs mirrors DefaultOutputMgr::RandomOutputFuncDefs.
// DefaultOutputMgr.cpp:154–163 — skip builtin; pure_rnd_upto file; Function::Output.
// Returns nFiles body strings. Incomplete funcs sticky nil.
func RandomOutputFuncDefs(funcs []*Function, nFiles int, forceStatic, funcAttr bool, rng *Rng) []string {
	if nFiles <= 0 {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	if !FunctionsComplete(funcs) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	out := make([]string, nFiles)
	for _, f := range funcs {
		if f.IsBuiltin {
			continue
		}
		idx := PureRndUptoIndex(nFiles)
		if idx < 0 || sessHasError(nil) {
			return nil
		}
		body := f.OutputOpts(forceStatic, funcAttr, rng)
		if sessHasError(nil) || body == "" {
			if !sessHasError(nil) {
				sessNoteError(nil, ErrGeneric)
			}
			return nil
		}
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		out[idx] += body
	}
	return out
}

// RandomOutputDefs mirrors DefaultOutputMgr::RandomOutputDefs.
// DefaultOutputMgr.cpp:165–168 — var defs then func defs into same nFiles buckets.
func RandomOutputDefs(globals []*Variable, funcs []*Function, nFiles int, forceStatic, funcAttr bool, rng *Rng) []string {
	vars := RandomOutputVarDefs(globals, nFiles, forceStatic)
	if vars == nil || sessHasError(nil) {
		return nil
	}
	fn := RandomOutputFuncDefs(funcs, nFiles, forceStatic, funcAttr, rng)
	if fn == nil || sessHasError(nil) {
		return nil
	}
	out := make([]string, nFiles)
	for i := 0; i < nFiles; i++ {
		out[i] = vars[i] + fn[i]
	}
	return out
}

// SplitAllHeadersContent mirrors DefaultOutputMgr::OutputAllHeaders for N files.
// DefaultOutputMgr.cpp:120–141 — secondary preambles; primary include; all get forwards.
// forwards is OutputForwardDeclarations text (same into every file).
func SplitAllHeadersContent(nFiles int, paranoid bool, forwards string) []string {
	if nFiles <= 0 {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	out := make([]string, nFiles)
	for i := 1; i < nFiles; i++ {
		out[i] = SplitSecondaryHeaderPreamble(paranoid)
	}
	out[0] = SplitPrimaryHeaderInclude()
	for i := 0; i < nFiles; i++ {
		out[i] += forwards
		if forwards != "" && !strings.HasSuffix(forwards, "\n") {
			out[i] += "\n"
		}
		out[i] += "\n"
	}
	return out
}

// DFSOutputHeader mirrors DFSOutputMgr::OutputHeader.
// DFSOutputMgr.cpp:63–66 — compact skips header entirely.
func DFSOutputHeader(header string, compact bool) string {
	if compact {
		return ""
	}
	return header
}

// DFSStructOutputPath mirrors DFSOutputMgr::struct_output_ path for structs file.
func DFSStructOutputPath() string {
	if currentSession().StructOutput == "" {
		return DefaultStructOutputName
	}
	return currentSession().StructOutput
}
