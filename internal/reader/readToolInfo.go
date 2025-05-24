
package reader

import (
	"os"
	"gopkg.in/yaml.v3"
	"github.com/Noswad123/mind-weaver/internal/parser"
)

func LoadToolYAML(path string) (*parser.ToolYAML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tool parser.ToolYAML
	if err := yaml.Unmarshal(data, &tool); err != nil {
		return nil, err
	}
	return &tool, nil
}
