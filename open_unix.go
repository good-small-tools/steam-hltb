//go:build !windows

package main

import (
	"os/exec"
	"runtime"
)

// openInBrowser は macOS / Linux で OS 既定のハンドラ（ブラウザ）で target を開く。
func openInBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	default: // linux など
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}
