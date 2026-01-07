package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"bakutil/internal/backup"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Perform backup of source directory",
	Long:  `Backup performs a complete backup of the source directory with intelligent exclusions.`,
	RunE:  runBackup,
}

var (
	destination string
)

func init() {
	rootCmd.AddCommand(backupCmd)
	backupCmd.Flags().StringVarP(&destination, "destination", "d", "", "destination directory for backup")
	backupCmd.MarkFlagRequired("destination")
}

func runBackup(cmd *cobra.Command, args []string) error {
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	// Override destination if provided
	if destination != "" {
		cfg.DestinationDir = destination
	}
	
	// Ensure destination is absolute
	if !filepath.IsAbs(cfg.DestinationDir) {
		abs, err := filepath.Abs(cfg.DestinationDir)
		if err != nil {
			return fmt.Errorf("invalid destination path: %w", err)
		}
		cfg.DestinationDir = abs
	}
	
	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(cfg.DestinationDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	
	log := getLogger()
	
	log.Info("Starting backup",
		"source", cfg.SourceDir,
		"destination", cfg.DestinationDir)
	
	// Create backup utility
	util := backup.NewUtility(cfg, log)
	
	// Perform complete backup
	if err := util.PerformBackup(); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	
	log.Info("Backup completed successfully", "backupPath", util.GetBackupPath())
	
	return nil
}