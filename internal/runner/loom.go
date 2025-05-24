package runner

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/urfave/cli/v2"
)

func RunLoomCommand(c *cli.Context, pythonPath, loomPath string) error {
	cmd := exec.Command(pythonPath, loomPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("🔍 Executing:", cmd.String())
	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Failed to run visualizer: %v\n", err)
	}
	return nil
}
