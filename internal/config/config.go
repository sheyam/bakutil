package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the backup utility configuration
type Config struct {
	IgnorePatterns          []string `mapstructure:"ignore_patterns" yaml:"ignore_patterns"`
	AlwaysInclude          []string `mapstructure:"always_include" yaml:"always_include"`
	LargeFileThresholdMB   int64    `mapstructure:"large_file_threshold_mb" yaml:"large_file_threshold_mb"`
	UseRsync               bool     `mapstructure:"use_rsync" yaml:"use_rsync"`
	EnableResume           bool     `mapstructure:"enable_resume" yaml:"enable_resume"`
	EnableHashVerification bool     `mapstructure:"enable_hash_verification" yaml:"enable_hash_verification"`
	SourceDir              string   `mapstructure:"source_dir" yaml:"source_dir"`
	DestinationDir         string   `mapstructure:"destination_dir" yaml:"destination_dir"`
}

// NewConfig creates a new configuration with sensible defaults
func NewConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	
	return &Config{
		IgnorePatterns:          DefaultIgnorePatterns(),
		AlwaysInclude:          DefaultAlwaysInclude(),
		LargeFileThresholdMB:   100,
		UseRsync:              true,
		EnableResume:          true,
		EnableHashVerification: true,
		SourceDir:             homeDir,
		DestinationDir:        filepath.Join(homeDir, "backups"),
	}
}

// DefaultIgnorePatterns returns the default set of patterns to ignore
func DefaultIgnorePatterns() []string {
	return []string{
		".cache", ".Trash", ".DS_Store", ".localized",
		"node_modules", ".npm", ".yarn", "venv", ".venv",
		"build", "dist", "__pycache__", "*.pyc",
		"Library/Caches", "Library/Application Support/Caches",
		"*.log", "*.tmp", "*.swp",
	}
}

// DefaultAlwaysInclude returns directories to always include
func DefaultAlwaysInclude() []string {
	return []string{
		"Documents", "Desktop", "Pictures", "Downloads",
		".ssh", ".gitconfig", ".bashrc", ".zshrc",
	}
}

// LoadConfig loads configuration from file or creates default
func LoadConfig(configPath string) (*Config, error) {
	config := NewConfig()
	
	if configPath == "" {
		homeDir, _ := os.UserHomeDir()
		candidates := []string{
			"backup.yaml",
			filepath.Join(homeDir, ".backup.yaml"),
		}
		
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				configPath = candidate
				break
			}
		}
	}
	
	if configPath == "" {
		return config, nil
	}
	
	viper.SetConfigFile(configPath)
	viper.AutomaticEnv()
	viper.SetEnvPrefix("BACKUP")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return config, nil
		}
		return nil, fmt.Errorf("error reading config file: %w", err)
	}
	
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}
	
	return config, nil
}