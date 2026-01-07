package backup

import (
	"fmt"
	"os"
	"path/filepath"

	"bakutil/internal/config"
	"bakutil/internal/types"
)

// simpleLogger implements a basic logger for standalone functions
type simpleLogger struct{}

func (l *simpleLogger) Info(msg string, args ...interface{}) {
	// Silent implementation for verification
}

func (l *simpleLogger) Warn(msg string, args ...interface{}) {
	// Silent implementation for verification  
}

func (l *simpleLogger) Error(msg string, args ...interface{}) {
	// Silent implementation for verification
}

func (l *simpleLogger) Debug(msg string, args ...interface{}) {
	// Silent implementation for verification
}

// VerificationEngine handles backup verification and comparison
type VerificationEngine struct {
	utility *Utility
}

// NewVerificationEngine creates a new verification engine
func NewVerificationEngine(utility *Utility) *VerificationEngine {
	return &VerificationEngine{
		utility: utility,
	}
}

// CompareManifests compares two file manifests and returns comparison results
func (ve *VerificationEngine) CompareManifests(preManifest, postManifest types.FileManifest) *types.BackupComparison {
	comparison := &types.BackupComparison{
		TotalFilesPre:   len(preManifest),
		TotalFilesPost:  len(postManifest),
		MissingFiles:    []string{},
		NewFiles:        []string{},
		ModifiedFiles:   []types.ModifiedFile{},
		SizeDifferences: []types.SizeDifference{},
	}
	
	// Check for missing files (in pre but not in post)
	for filePath, preInfo := range preManifest {
		if postInfo, exists := postManifest[filePath]; !exists {
			comparison.MissingFiles = append(comparison.MissingFiles, filePath)
		} else {
			// Check if files are identical
			if preInfo.SHA256 != "" && postInfo.SHA256 != "" {
				if preInfo.SHA256 == postInfo.SHA256 && preInfo.Size == postInfo.Size {
					comparison.IdenticalFiles++
				} else {
					// File modified
					comparison.ModifiedFiles = append(comparison.ModifiedFiles, types.ModifiedFile{
						Path:     filePath,
						Reason:   "Hash or size difference",
						OldHash:  preInfo.SHA256,
						NewHash:  postInfo.SHA256,
						OldSize:  preInfo.Size,
						NewSize:  postInfo.Size,
						OldMTime: preInfo.MTime.Format("2006-01-02 15:04:05"),
						NewMTime: postInfo.MTime.Format("2006-01-02 15:04:05"),
					})
				}
			} else if preInfo.Size != postInfo.Size {
				// Size difference without hashes
				comparison.SizeDifferences = append(comparison.SizeDifferences, types.SizeDifference{
					Path:    filePath,
					OldSize: preInfo.Size,
					NewSize: postInfo.Size,
				})
			} else {
				comparison.IdenticalFiles++
			}
		}
	}
	
	// Check for new files (in post but not in pre)
	for filePath := range postManifest {
		if _, exists := preManifest[filePath]; !exists {
			comparison.NewFiles = append(comparison.NewFiles, filePath)
		}
	}
	
	return comparison
}

// GenerateVerificationReport creates a detailed verification report
func (ve *VerificationEngine) GenerateVerificationReport(preManifest, postManifest types.FileManifest, comparison *types.BackupComparison) error {
	reportPath := filepath.Join(ve.utility.backupPath, "verification_report.txt")
	
	file, err := os.Create(reportPath)
	if err != nil {
		return fmt.Errorf("failed to create verification report: %w", err)
	}
	defer file.Close()
	
	fmt.Fprintf(file, "============================================================\n")
	fmt.Fprintf(file, "BACKUP VERIFICATION REPORT\n")
	fmt.Fprintf(file, "============================================================\n\n")
	fmt.Fprintf(file, "Verification Date: %s\n\n", ve.utility.stats.EndTime.Format("2006-01-02 15:04:05"))
	
	fmt.Fprintf(file, "SUMMARY:\n")
	fmt.Fprintf(file, "------------------------------------------------------------\n")
	fmt.Fprintf(file, "Files in source: %d\n", comparison.TotalFilesPre)
	fmt.Fprintf(file, "Files in backup: %d\n", comparison.TotalFilesPost)
	fmt.Fprintf(file, "Identical files: %d\n", comparison.IdenticalFiles)
	fmt.Fprintf(file, "Missing files: %d\n", len(comparison.MissingFiles))
	fmt.Fprintf(file, "New files: %d\n", len(comparison.NewFiles))
	fmt.Fprintf(file, "Modified files: %d\n", len(comparison.ModifiedFiles))
	fmt.Fprintf(file, "Size differences: %d\n\n", len(comparison.SizeDifferences))
	
	// Missing files
	if len(comparison.MissingFiles) > 0 {
		fmt.Fprintf(file, "MISSING FILES:\n")
		fmt.Fprintf(file, "------------------------------------------------------------\n")
		for _, filePath := range comparison.MissingFiles {
			fmt.Fprintf(file, "  %s\n", filePath)
		}
		fmt.Fprintf(file, "\n")
	}
	
	// New files
	if len(comparison.NewFiles) > 0 {
		fmt.Fprintf(file, "NEW FILES (in backup but not source):\n")
		fmt.Fprintf(file, "------------------------------------------------------------\n")
		for _, filePath := range comparison.NewFiles {
			fmt.Fprintf(file, "  %s\n", filePath)
		}
		fmt.Fprintf(file, "\n")
	}
	
	// Modified files
	if len(comparison.ModifiedFiles) > 0 {
		fmt.Fprintf(file, "MODIFIED FILES:\n")
		fmt.Fprintf(file, "------------------------------------------------------------\n")
		for _, mf := range comparison.ModifiedFiles {
			fmt.Fprintf(file, "  %s (%s)\n", mf.Path, mf.Reason)
			if mf.OldHash != "" && mf.NewHash != "" {
				fmt.Fprintf(file, "    Old hash: %s\n", mf.OldHash)
				fmt.Fprintf(file, "    New hash: %s\n", mf.NewHash)
			}
			if mf.OldSize != mf.NewSize {
				fmt.Fprintf(file, "    Old size: %d bytes\n", mf.OldSize)
				fmt.Fprintf(file, "    New size: %d bytes\n", mf.NewSize)
			}
		}
		fmt.Fprintf(file, "\n")
	}
	
	// Overall status
	success := len(comparison.MissingFiles) == 0 && len(comparison.ModifiedFiles) == 0
	if success {
		fmt.Fprintf(file, "VERIFICATION RESULT: ✓ SUCCESS\n")
		fmt.Fprintf(file, "All files were backed up successfully.\n")
	} else {
		fmt.Fprintf(file, "VERIFICATION RESULT: ⚠ ISSUES FOUND\n")
		fmt.Fprintf(file, "Some files may not have been backed up correctly.\n")
	}
	
	ve.utility.logger.Info("Verification report generated", "path", reportPath)
	return nil
}

// VerifyBackup performs complete backup verification
func (ve *VerificationEngine) VerifyBackup(backupPath string) (*types.VerificationResult, error) {
	// Load pre-backup manifest
	preManifestPath := filepath.Join(backupPath, "pre_backup_manifest.json")
	preManifest, err := LoadManifest(preManifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load pre-backup manifest: %w", err)
	}
	
	ve.utility.logger.Info("Loaded pre-backup manifest", "files", len(preManifest))
	
	// Generate post-backup manifest
	backupTargetDir := filepath.Join(backupPath, filepath.Base(ve.utility.config.SourceDir))
	if _, err := os.Stat(backupTargetDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("backup target directory not found: %s", backupTargetDir)
	}
	
	ve.utility.logger.Info("Generating post-backup manifest", "directory", backupTargetDir)
	postManifest, err := ve.utility.GenerateFileManifest(backupTargetDir, ve.utility.config.EnableHashVerification)
	if err != nil {
		return nil, fmt.Errorf("failed to generate post-backup manifest: %w", err)
	}
	
	// Save post-backup manifest
	if err := ve.utility.SaveManifest(postManifest, "post_backup_manifest.json"); err != nil {
		return nil, fmt.Errorf("failed to save post-backup manifest: %w", err)
	}
	
	// Compare manifests
	comparison := ve.CompareManifests(preManifest, postManifest)
	
	// Generate verification report
	if err := ve.GenerateVerificationReport(preManifest, postManifest, comparison); err != nil {
		return nil, fmt.Errorf("failed to generate verification report: %w", err)
	}
	
	// Determine if verification passed
	success := len(comparison.MissingFiles) == 0 && len(comparison.ModifiedFiles) == 0
	issues := []string{}
	
	if len(comparison.MissingFiles) > 0 {
		issues = append(issues, fmt.Sprintf("%d files missing from backup", len(comparison.MissingFiles)))
	}
	if len(comparison.ModifiedFiles) > 0 {
		issues = append(issues, fmt.Sprintf("%d files have differences", len(comparison.ModifiedFiles)))
	}
	
	result := &types.VerificationResult{
		Success:       success,
		TotalFiles:    len(preManifest),
		VerifiedFiles: comparison.IdenticalFiles,
		Issues:        issues,
		Comparison:    comparison,
		ReportPath:    filepath.Join(backupPath, "verification_report.txt"),
	}
	
	return result, nil
}

// CompleteBackupVerification completes verification for a backup that was interrupted
func CompleteBackupVerification(backupPath, sourceDir string, generateHashes bool) (*types.VerificationResult, error) {
	// This is a standalone function for the verification utility
	
	// Load pre-backup manifest
	preManifestPath := filepath.Join(backupPath, "pre_backup_manifest.json")
	preManifest, err := LoadManifest(preManifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load pre-backup manifest: %w", err)
	}
	
	fmt.Printf("✓ Loaded pre-backup manifest: %d files\n", len(preManifest))
	
	// Find backup target directory
	backupTargetDir := filepath.Join(backupPath, filepath.Base(sourceDir))
	if _, err := os.Stat(backupTargetDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("backup target directory not found: %s", backupTargetDir)
	}
	
	fmt.Printf("Backup target: %s\n", backupTargetDir)
	
	// Create a temporary utility for manifest generation
	cfg := &config.Config{
		SourceDir:              sourceDir,
		DestinationDir:         filepath.Dir(backupPath),
		EnableHashVerification: generateHashes,
		IgnorePatterns:         config.DefaultIgnorePatterns(),
		AlwaysInclude:          config.DefaultAlwaysInclude(),
	}
	
	// Create a simple logger for this function
	log := &simpleLogger{}
	
	utility := NewUtility(cfg, log)
	utility.backupPath = backupPath
	
	// Generate post-backup manifest
	fmt.Println("Generating post-backup manifest...")
	postManifest, err := utility.GenerateFileManifest(backupTargetDir, generateHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate post-backup manifest: %w", err)
	}
	
	// Save post-backup manifest
	if err := utility.SaveManifest(postManifest, "post_backup_manifest.json"); err != nil {
		return nil, fmt.Errorf("failed to save post-backup manifest: %w", err)
	}
	
	// Compare manifests
	ve := NewVerificationEngine(utility)
	comparison := ve.CompareManifests(preManifest, postManifest)
	
	// Generate verification report
	if err := ve.GenerateVerificationReport(preManifest, postManifest, comparison); err != nil {
		return nil, fmt.Errorf("failed to generate verification report: %w", err)
	}
	
	// Determine if verification passed
	success := len(comparison.MissingFiles) == 0 && len(comparison.ModifiedFiles) == 0
	return &types.VerificationResult{
		Success:    success,
		TotalFiles: len(preManifest),
		Comparison: comparison,
		ReportPath: filepath.Join(backupPath, "verification_report.txt"),
	}, nil
}