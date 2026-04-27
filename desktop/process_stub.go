//go:build !windows

package main

import "os/exec"

func configureBackgroundCmd(cmd *exec.Cmd) {
	_ = cmd
}

func configureWindowedAppCmd(cmd *exec.Cmd) {
	_ = cmd
}
