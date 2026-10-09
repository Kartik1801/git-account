// Package sysx opens web pages and uses the clipboard on each OS.
package sysx

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/atotto/clipboard"
)

// OpenURL opens u in the default browser without waiting for it.
func OpenURL(u string) error {
	var cmd *exec.Cmd
	switch {
	case runtime.GOOS == "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case runtime.GOOS == "darwin":
		cmd = exec.Command("open", u)
	case IsWSL():
		if p, err := exec.LookPath("wslview"); err == nil {
			cmd = exec.Command(p, u)
		} else {
			cmd = exec.Command("cmd.exe", "/c", "start", "", strings.ReplaceAll(u, "&", "^&"))
		}
	default:
		cmd = exec.Command("xdg-open", u)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// Copy puts text on the clipboard.
func Copy(text string) error {
	if IsWSL() {
		if clip, err := exec.LookPath("clip.exe"); err == nil {
			cmd := exec.Command(clip)
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		}
	}
	return clipboard.WriteAll(text)
}

// IsWSL reports whether this is Linux running inside WSL.
func IsWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	b, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	return strings.Contains(strings.ToLower(string(b)), "microsoft")
}
