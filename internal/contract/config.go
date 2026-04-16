package contract

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the barometer config file (e.g. barometer.yaml).
type Config struct {
	OpenAPI  *OpenAPIConfig `yaml:"openapi,omitempty" json:"openapi,omitempty"`
	Arazzo   *ArazzoConfig  `yaml:"arazzo,omitempty" json:"arazzo,omitempty"`
	BaseURL  string         `yaml:"baseUrl,omitempty" json:"baseUrl,omitempty"`
	ProxyURL string         `yaml:"proxyUrl,omitempty" json:"proxyUrl,omitempty"`
	Output   string         `yaml:"output,omitempty" json:"output,omitempty"` // human, junit, json
}

// OpenAPIConfig specifies OpenAPI contract test run.
type OpenAPIConfig struct {
	Spec string   `yaml:"spec" json:"spec"`
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`
}

// ArazzoConfig specifies Arazzo workflow run.
type ArazzoConfig struct {
	Doc       string   `yaml:"doc" json:"doc"`
	Workflows []string `yaml:"workflows,omitempty" json:"workflows,omitempty"` // empty = all
}

// LoadConfig reads a config file from path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return &c, nil
}
