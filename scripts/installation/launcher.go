package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	// Check if cloud-installer.go exists in the installer directory
	installerPath := filepath.Join("installer", "cloud-installer.go")
	if _, err := os.Stat(installerPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: %s not found\n", installerPath)
		fmt.Fprintf(os.Stderr, "Make sure you're running this from the scripts/installation directory\n")
		os.Exit(1)
	}
	
	// Prepare command arguments
	args := append([]string{"run", installerPath}, os.Args[1:]...)
	
	// Execute the cloud installer
	cmd := exec.Command("go", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error running installer: %v\n", err)
		os.Exit(1)
	}
}
