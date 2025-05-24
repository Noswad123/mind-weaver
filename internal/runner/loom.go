package runner

import (
	"os"
	"github.com/urfave/cli/v2"
	"os/exec"
)

func RunLoomCommand(c *cli.Context) error {
	python := os.Getenv("PYTHON_PATH")
	if python == "" {
		python = "python3"
	}
	cmd := exec.Command(python, "scripts/loom/main.py")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		print("❌ Failed to run visualizer: %v", err)
	}
	return nil
}
