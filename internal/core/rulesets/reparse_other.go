//go:build !windows

package rulesets

import "os"

func isReparse(info os.FileInfo) bool { return info.Mode()&os.ModeSymlink != 0 }
