package main

import (
	"os"
	"os/exec"
	"strings"
)

func isBrokerdLaunched() (bool, error) {
	cmd := "ps -ef | grep brokerd | grep -v grep"
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(out), "brokerd"), nil
}

func canBeRun(command string) (bool, error) {
	cmd := "which " + command
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(out), command), nil
}

func runOnlyInIntegrationTest(key string) {
	if tenant_override := os.Getenv(key); tenant_override == "" {
		panic("This only runs as an integration test")
	}
}
