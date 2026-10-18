//go:build !windows && !linux && !darwin

package main

import "os/exec"

func configureProcess(cmd *exec.Cmd) {}
