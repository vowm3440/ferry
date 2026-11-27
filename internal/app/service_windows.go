//go:build windows

package app

import (
	"errors"
	"ferry/internal/ipc"
	"ferry/internal/release"
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
	installed, running, needsRepair, startType, err := ipc.InspectService(c.guiExe)
	if err != nil {
		return ServiceStatus{StartType: "disabled"}, err
	}
	return ServiceStatus{
		Installed:   installed,
		Running:     running,
		NeedsRepair: needsRepair,
		StartType:   startType,
		Version:     release.Version,
	}, nil
}

func (c *WindowsServiceController) EnsureRunning() error {
	if err := ipc.StartWindowsService(ipc.ServiceName); err != nil {
		return err
	}
	installed, running, needsRepair, _, err := ipc.InspectService(c.guiExe)
	if err != nil || !installed || !running || needsRepair {
		return errors.New("系统服务异常或需要更新，请修复服务")
	}
	if c.onModeChanged != nil {
		_ = c.onModeChanged()
	}
	return nil
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
	if c.onModeChanged != nil {
		_ = c.onModeChanged()
	}
	if err := ipc.RunElevated(c.coreExe, args); err != nil {
		return err
	}
	if c.onModeChanged != nil {
		_ = c.onModeChanged()
	}
	return nil
}
