package main

import (
	"fmt"
	"os"
	"os/exec"
	goruntime "runtime"
)

// OpenStylesFolder shows the folder of the page styles in the system's file manager (desktop only).
func (a *App) OpenStylesFolder() error {
	folder := a.StylesFolder()
	if folder == "" {
		return fmt.Errorf("there is no stylesheets folder")
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", folder)
	case "windows":
		cmd = exec.Command("explorer", folder)
	default:
		cmd = exec.Command("xdg-open", folder)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // explorer reports 1 even when it worked, so the result is not looked at
	return nil
}
