//go:build !windows

package app

import "os/exec"

func launchInstallerExecutable(clean string) error {
	return exec.Command(clean).Start()
}
