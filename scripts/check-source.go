// Command check-source enforces the agreed Go source size limits.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	failed := false
	for _, root := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if !check(path) {
				failed = true
			}
			return nil
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func check(path string) bool {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, nil, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	valid := true
	if set.Position(file.End()).Line > 800 {
		fmt.Printf("%s exceeds 800 lines\n", path)
		valid = false
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		length := set.Position(fn.End()).Line - set.Position(fn.Pos()).Line + 1
		if length > 50 {
			fmt.Printf("%s:%d %s has %d lines (limit 50)\n", path, set.Position(fn.Pos()).Line, fn.Name.Name, length)
			valid = false
		}
	}
	return valid
}
