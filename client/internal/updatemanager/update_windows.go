//go:build windows

package updatemanager

import (
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"

	"golang.org/x/sys/windows/registry"
)

const (
	msiDownloadURL = "https://github.com/netbirdio/netbird/releases/download/v%s/netbird_installer_%s_windows_amd64.msi"
	exeDownloadURL = "https://github.com/netbirdio/netbird/releases/download/v%s/netbird_installer_%s_windows_amd64.exe"
)

func (u *UpdateManager) triggerUpdate(targetVersion string) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\Netbird`, registry.QUERY_VALUE)
	if err != nil && strings.Contains(err.Error(), "system cannot find the file specified") {
		// Installed using MSI installer
		path, err := downloadFileToTemporaryDir(strings.ReplaceAll(msiDownloadURL, "%s", targetVersion))
		if err != nil {
			return err
		}
		cmd := exec.Command("msiexec", "/quiet", "/i", path)
		err = cmd.Run()
		return err
	} else if err != nil {
		return err
	}
	err = k.Close()
	if err != nil {
		log.Warnf("Error closing registry key: %v", err)
	}

	// Installed using EXE installer
	path, err := downloadFileToTemporaryDir(strings.ReplaceAll(exeDownloadURL, "%s", targetVersion))
	if err != nil {
		return err
	}
	cmd := exec.Command(path, "/S")
	err = cmd.Start()
	if err != nil {
		return err
	}
	err = cmd.Process.Release()

	return err
}
