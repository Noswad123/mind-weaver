
package runner

import (
	// "os"
	"log"

	"github.com/urfave/cli/v2"

	"github.com/Noswad123/mind-weaver/internal/db"
	// "github.com/Noswad123/mind-weaver/internal/interactive"
	"github.com/Noswad123/mind-weaver/internal/reader"
)


func RunReciteCommand(c *cli.Context, cheatDb *db.CheatDb) error {
yamlPath := "/Users/noswadian/Projects/mind-weaver/yaml/git.yaml"
tool, err := reader.LoadToolYAML(yamlPath)
if err != nil {
	log.Fatal("Failed to parse YAML:", err)
}

	if err := cheatDb.InsertToolYAML(tool); err != nil {
	log.Fatal("Failed to upload to DB:", err)
}
	// args := c.Args().Slice()
	// hasNoArgs := len(args) == 0
	//
	// if hasNoArgs {
	// 	if err := interactive.RunCheatTUI(cheatDb); err != nil {
	// 		log.Fatalf("Failed to start TUI: %v", err)
	// 	}
	// 	os.Exit(0)
	// }

	return nil
}
