package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/victor141516/besht/internal/codegen"
	"github.com/victor141516/besht/internal/stdlib"
	"github.com/victor141516/besht/internal/viewer"
)

const usage = `besht — TypeScript-flavored shell compiler

Usage:
  besht compile <file.bsh>                    Compile and print to stdout
  besht compile <file.bsh> -o <out.sh>        Compile to single file
  besht compile <file.bsh> --split -o <dir/>  Compile each file separately into <dir/>
  besht compile --check <file.bsh>            Validate imports, command usage, and unsupported fetch APIs (no output)
  besht visualize <file.bsh>                  Open a terminal side-by-side source/shell view
  besht init                          Write ./stdlib.d.bsh declarations
  besht init --force                  Overwrite ./stdlib.d.bsh declarations
  besht --version                     Print version

Compatibility:
  besht <file.bsh> [flags]            Alias for besht compile <file.bsh> [flags]
  besht --check <file.bsh>            Alias for besht compile --check <file.bsh>

Flags:
  -o <path>    Output file (default: stdout) or output directory (with --split)
  --split      Compile each .bsh file to its own .sh file; imports become 'source' calls
  --opt-no-add-binaries-check   Omit the runtime POSIX self-check from compiled output
  --opt-no-source-map            Omit # besht:file:line:col source comments from compiled output
  --opt-resolve-ts-imports       Resolve extensionless imports to .ts when .bsh is absent
  --opt-allow-external-shell-imports  Allow explicit .sh imports outside the compiler root
  --opt-use-jq                  Allow generated JSON code to depend on jq
  --check      Validate imports, command usage, and unsupported fetch APIs; do not generate output
  --version    Show version and exit
  -h, --help   Show this message

Visualize:
  Uses bat for TypeScript and shell syntax highlighting when bat is installed and output is a terminal.
`

const version = "0.1.0"

type cliConfig struct {
	inputFile                 string
	outputPath                string
	checkOnly                 bool
	splitMode                 bool
	noCheck                   bool
	noSourceMap               bool
	resolveTSImports          bool
	allowExternalShellImports bool
	useJQ                     bool
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Print(usage)
		os.Exit(0)
	}
	if args[0] == "--version" {
		fmt.Println("besht", version)
		os.Exit(0)
	}
	if args[0] == "init" {
		if err := runInit(args[1:], ".", os.Stderr); err != nil {
			fatal("%s", err)
		}
		return
	}
	if args[0] == "compile" {
		runCompile(args[1:])
		return
	}
	if args[0] == "visualize" {
		runVisualize(args[1:])
		return
	}

	runCompile(args)
}

func runCompile(args []string) {
	cfg := parseArgs(args, "compile", true, true, true)
	validateInputFile(cfg.inputFile)
	opts := cfg.codegenOptions()

	if cfg.checkOnly {
		runCheck(cfg.inputFile, opts)
		return
	}

	if cfg.splitMode {
		if cfg.outputPath == "" {
			fatal("--split requires -o <output-directory>")
		}
		runCompileSplit(cfg.inputFile, cfg.outputPath, opts)
		return
	}

	runCompileFile(cfg.inputFile, cfg.outputPath, opts)
}

func runVisualize(args []string) {
	cfg := parseArgs(args, "visualize", false, false, false)
	validateInputFile(cfg.inputFile)
	out, err := viewer.BuildWithOptions(cfg.inputFile, cfg.codegenOptions(), terminalWidth(), visualRenderOptions())
	if err != nil {
		fatal("%s", err)
	}
	if err := showInTerminal(out); err != nil {
		fatal("visualize failed: %s", err)
	}
}

func parseArgs(args []string, mode string, allowOutput, allowSplit, allowCheck bool) cliConfig {
	var cfg cliConfig
	var inputFile string
	var outputPath string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--check":
			if !allowCheck {
				fatal("%s mode does not support --check", mode)
			}
			cfg.checkOnly = true
		case "--split":
			if !allowSplit {
				fatal("%s mode does not support --split", mode)
			}
			cfg.splitMode = true
		case "--opt-no-add-binaries-check":
			cfg.noCheck = true
		case "--opt-no-source-map":
			cfg.noSourceMap = true
		case "--opt-resolve-ts-imports":
			cfg.resolveTSImports = true
		case "--opt-allow-external-shell-imports":
			cfg.allowExternalShellImports = true
		case "--opt-use-jq":
			cfg.useJQ = true
		case "-o":
			if !allowOutput {
				fatal("%s mode does not write output files", mode)
			}
			i++
			if i >= len(args) {
				fatal("-o requires a path argument")
			}
			outputPath = args[i]
		default:
			if strings.HasPrefix(args[i], "-") {
				fatal("unknown flag: %s", args[i])
			}
			if inputFile != "" {
				fatal("multiple input files not supported; use import statements")
			}
			inputFile = args[i]
		}
	}

	if inputFile == "" {
		fatal("no input file specified\n\n%s", usage)
	}
	cfg.inputFile = inputFile
	cfg.outputPath = outputPath
	return cfg
}

func validateInputFile(inputFile string) {
	if !strings.HasSuffix(inputFile, ".bsh") {
		fmt.Fprintf(os.Stderr, "warning: input file %q does not have .bsh extension\n", inputFile)
	}
}

func (cfg cliConfig) codegenOptions() codegen.Options {
	return codegen.Options{
		NoCheck:                   cfg.noCheck,
		NoSourceMap:               cfg.noSourceMap,
		ResolveTsImports:          cfg.resolveTSImports,
		AllowExternalShellImports: cfg.allowExternalShellImports,
		UseJQ:                     cfg.useJQ,
	}
}

func runCheck(inputFile string, opts codegen.Options) {
	if err := checkFile(inputFile, opts); err != nil {
		fatal("%s", err)
	}
	fmt.Fprintf(os.Stderr, "%s: OK\n", inputFile)
}

func checkFile(inputFile string, opts codegen.Options) error {
	return codegen.CheckFile(inputFile, opts)
}

func runInit(args []string, dir string, stderr io.Writer) error {
	force := false
	for _, arg := range args {
		if arg != "--force" {
			return fmt.Errorf("unsupported init argument: %s", arg)
		}
		if force {
			return fmt.Errorf("unsupported init argument: %s", arg)
		}
		force = true
	}

	path := "stdlib.d.bsh"
	if dir != "" && dir != "." {
		path = dir + string(os.PathSeparator) + path
	}
	content := []byte(stdlib.Declarations)
	existing, err := os.ReadFile(path)
	if err == nil {
		if string(existing) == stdlib.Declarations {
			fmt.Fprintln(stderr, "stdlib.d.bsh already up to date")
			return nil
		}
		if !force {
			return fmt.Errorf("stdlib.d.bsh already exists; pass --force to overwrite")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot read stdlib.d.bsh: %s", err)
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("cannot write stdlib.d.bsh: %s", err)
	}
	fmt.Fprintln(stderr, "wrote stdlib.d.bsh")
	return nil
}

func runCompileFile(inputFile, outputFile string, opts codegen.Options) {
	out, err := codegen.CompileFile(inputFile, opts)
	if err != nil {
		fatal("%s", err)
	}

	if outputFile == "" {
		fmt.Print(out)
		return
	}

	if err := os.WriteFile(outputFile, []byte(out), 0755); err != nil {
		fatal("cannot write %s: %s", outputFile, err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", outputFile)
}

func runCompileSplit(inputFile, outDir string, opts codegen.Options) {
	if err := codegen.CompileFileSplit(inputFile, outDir, opts); err != nil {
		fatal("%s", err)
	}
	fmt.Fprintf(os.Stderr, "wrote split output to %s\n", outDir)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "besht: error: "+format+"\n", args...)
	os.Exit(1)
}
