package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"bakutil/internal/config"
	"bakutil/pkg/logger"
)

var (
	cfgFile  string
	verbose  bool
	jsonOut  bool
	rootCmd  = &cobra.Command{
		Use:   "backup-util",
		Short: "A comprehensive backup utility",
		Long:  `A production-ready backup utility with intelligent exclusions and verification.`,
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.backup.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "JSON output format")
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		
		// Search for config in home directory
		cfgFile = home + "/.backup.yaml"
	}
}

// getConfig loads the configuration
func getConfig() (*config.Config, error) {
	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, nil
}

// getLogger creates a logger based on flags
func getLogger() logger.Logger {
	format := "text"
	if jsonOut {
		format = "json"
	}
	
	level := "info"
	if verbose {
		level = "debug"
	}
	
	return logger.NewLogger(format, level)
}