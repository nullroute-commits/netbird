//go:build darwin

package updatemanager

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
)

const (
	pkgDownloadURL = "https://github.com/netbirdio/netbird/releases/download/v%version/netbird_%version_darwin_%arch.pkg"
)

func (u *UpdateManager) triggerUpdate(targetVersion string) error {
	cmd := exec.Command("pkgutil", "--pkg-info", "io.netbird.client")
	outBytes, err := cmd.Output()
	if err != nil && cmd.ProcessState.ExitCode() == 1 {
		// Not installed using pkg file, thus installed using Homebrew

		return u.updateHomeBrew()
	}
	// Installed using pkg file
	url := strings.ReplaceAll(pkgDownloadURL, "%version", targetVersion)
	url = strings.ReplaceAll(url, "%arch", runtime.GOARCH)
	path, err := downloadFileToTemporaryDir(url)
	if err != nil {
		return err
	}

	volume := "/"
	for _, v := range strings.Split(string(outBytes), "\n") {
		trimmed := strings.TrimSpace(v)
		if strings.HasPrefix(trimmed, "volume: ") {
			volume = strings.Split(trimmed, ": ")[1]
		}
	}

	cmd = exec.Command("installer", "-pkg", path, "-target", volume)

	err = cmd.Start()
	if err != nil {
		return err
	}
	err = cmd.Process.Release()

	return err
}

func (u *UpdateManager) updateHomeBrew() error {
	// Homebrew must be run as a non-root user
	// To find out which user installed NetBird using HomeBrew we can check the owner of our brew tap directory
	fileInfo, err := os.Stat("/opt/homebrew/Library/Taps/netbirdio/homebrew-tap/")
	if err != nil {
		return err
	}

	fileSysInfo, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("Error checking file owner, sysInfo type is %T not *syscall.Stat_t", fileInfo.Sys())
	}

	// Get user name from UID
	cmd := exec.Command("id", "-nu", fmt.Sprintf("%d", fileSysInfo.Uid))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	userName := strings.TrimSpace(string(out))

	// Get user HOME, required for brew to run correctly
	// https://github.com/Homebrew/brew/issues/15833
	cmd = exec.Command("sudo", "-u", userName, "sh", "-c", "echo $HOME")
	out, err = cmd.CombinedOutput()
	if err != nil {
		return err
	}

	homeDir := strings.TrimSpace(string(out))
	// Homebrew does not support installing specific versions
	// Thus it will always update to latest and ignore targetVersion
	cmd = exec.Command("sudo", "-u", userName, "/opt/homebrew/bin/brew", "upgrade", "netbirdio/tap/netbird")
	cmd.Env = append(cmd.Env, "HOME="+homeDir)

	// Homebrew upgrade doesn't restart the client on its own
	// So we have to wait for it to finish running and ensure it's done
	// And then basically restart the netbird service
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Errorf("Error running brew upgrade, output: %v", string(out))
		return err
	}

	currentPID := os.Getpid()

	// Restart netbird service after the fact
	// This is a workaround since attempting to restart using launchctl will kill the service and die before starting
	// the service again as it's a child process
	// using SigTerm should ensure a clean shutdown
	cmd = exec.Command("kill", "-15", fmt.Sprintf("%d", currentPID))
	err = cmd.Run()
	// We're dying now, which should restart us

	return err
}
