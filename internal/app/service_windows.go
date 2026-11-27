//go:build windows

package app

import (
	"ferry/internal/ipc"
	"path/filepath"
)

// WindowsServiceController uses Clash Verge Rev's UAC ("runas") elevation flow
// to install, repair, and uninstall FerryCore without Authenticode SPKI pins.
type WindowsServiceController struct {
	guiExe        string
	coreExe       string
	onModeChanged func() error
}

func NewWindowsServiceController(guiExe, coreExe string, onModeChanged func() error) *WindowsServiceController {
	return &WindowsServiceController{
		guiExe:        filepath.Clean(guiExe),
		coreExe:       filepath.Clean(coreExe),
		onModeChanged: onModeChanged,
	}
}

func (c *WindowsServiceController) Status() (ServiceStatus, error) {
	info, err := ipc.QueryServiceStatus(ipc.ServiceName)
	if err != nil {
		return ServiceStatus{StartType: "disabled"}, err
	}
	return ServiceStatus{
		Installed: info.Installed,
		Running:   info.Running,
		StartType: info.StartType,
		Version:   currentAppVersion,
	}, nil
}

func (c *WindowsServiceController) Install() error {
	return c.runElevated("install-service", true)
}

func (c *WindowsServiceController) Repair() error {
	return c.runElevated("repair-service", true)
}

func (c *WindowsServiceController) Uninstall() error {
	return c.runElevated("uninstall-service", false)
}

func (c *WindowsServiceController) runElevated(subcommand string, withIdentity bool) error {
	args := []string{subcommand}
	if withIdentity {
		sid, err := ipc.CurrentUserSID()
		if err != nil {
			return err
		}
		args = append(args, "--user-sid", sid, "--gui-exe", c.guiExe)
	}
	if err := ipc.RunElevated(c.coreExe, args); err != nil {
		return err
	}
	if c.onModeChanged != nil {
		_ = c.onModeChanged()
	}
	return nil
}
