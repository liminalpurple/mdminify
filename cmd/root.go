// Package cmd implements the mdminify command-line interface.
//
// The CLI reads markdown from files, directories, or stdin, minifies
// it using [github.com/liminalpurple/mdminify/minify], and writes the
// result to stdout or back to the source files.
package cmd

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/liminalpurple/mdminify/minify"
)

var version = "dev"

// Run executes the mdminify CLI with the given arguments and I/O streams.
// It returns an exit code: 0 on success, 1 on error or when -check detects
// changes.
//
// Flags:
//
//   - -w: write minified output back to source files (only if changed)
//   - -check: exit 1 if any file would be modified (for CI / pre-commit)
//   - -ext: comma-separated file extensions for directory mode (default ".md,.markdown")
//   - -v: print version
//
// With no path arguments, Run reads from stdin and writes to stdout.
// Directory arguments are walked recursively, processing files matching -ext.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mdminify", flag.ContinueOnError)
	fs.SetOutput(stderr)

	write := fs.Bool("w", false, "modify files in-place")
	check := fs.Bool("check", false, "exit 1 if any file would change")
	ext := fs.String("ext", ".md,.markdown", "file extensions for directory mode")
	showVersion := fs.Bool("v", false, "show version")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *showVersion {
		_, _ = fmt.Fprintf(stdout, "mdminify %s\n", version)
		return 0
	}

	paths := fs.Args()
	exts := parseExts(*ext)

	// No paths — read stdin, write stdout.
	if len(paths) == 0 {
		if *write {
			_, _ = fmt.Fprintln(stderr, "mdminify: -w cannot be used with stdin")
			return 1
		}
		if err := minify.Minify(stdin, stdout); err != nil {
			_, _ = fmt.Fprintf(stderr, "mdminify: %v\n", err)
			return 1
		}
		return 0
	}

	changed := false
	errored := false

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "mdminify: %v\n", err)
			errored = true
			continue
		}
		if info.IsDir() {
			err = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				if !matchesExt(path, exts) {
					return nil
				}
				c, e := processFile(path, *write, *check, stdout)
				if e != nil {
					_, _ = fmt.Fprintf(stderr, "mdminify: %s: %v\n", path, e)
					errored = true
				}
				if c {
					changed = true
				}
				return nil
			})
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "mdminify: %v\n", err)
				errored = true
			}
		} else {
			c, e := processFile(p, *write, *check, stdout)
			if e != nil {
				_, _ = fmt.Fprintf(stderr, "mdminify: %s: %v\n", p, e)
				errored = true
			}
			if c {
				changed = true
			}
		}
	}

	if errored {
		return 1
	}
	if *check && changed {
		return 1
	}
	return 0
}

func processFile(path string, write, check bool, stdout io.Writer) (changed bool, err error) {
	input, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	var buf strings.Builder
	if err := minify.Minify(strings.NewReader(string(input)), &buf); err != nil {
		return false, err
	}
	output := buf.String()

	if output == string(input) {
		return false, nil
	}

	if check {
		_, _ = fmt.Fprintf(stdout, "%s\n", path)
		return true, nil
	}

	if write {
		if err := os.WriteFile(path, []byte(output), 0644); err != nil {
			return false, err
		}
		return true, nil
	}

	_, err = io.WriteString(stdout, output)
	return true, err
}

func parseExts(s string) []string {
	parts := strings.Split(s, ",")
	exts := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, ".") {
			p = "." + p
		}
		exts = append(exts, p)
	}
	return exts
}

func matchesExt(path string, exts []string) bool {
	e := filepath.Ext(path)
	for _, x := range exts {
		if strings.EqualFold(e, x) {
			return true
		}
	}
	return false
}
