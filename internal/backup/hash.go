package backup

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bakutil/internal/config"
	"bakutil/internal/types"
	"bakutil/pkg/logger"
)

// HashEngine handles file hash generation and verification
type HashEngine struct {
	logger logger.Logger
}

// NewHashEngine creates a new hash engine
func NewHashEngine(log logger.Logger) *HashEngine {
	return &HashEngine{
		logger: log,
	}
}

// GenerateFileHash generates SHA256 hash for a file
func (he *HashEngine) GenerateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for hashing: %w", err)
	}
	defer file.Close()
	
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}
	
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// GenerateHashManifest creates a hash manifest for a directory
func (he *HashEngine) GenerateHashManifest(rootPath string, manifest types.FileManifest, cfg *config.Config) error {
	he.logger.Info("Generating hash manifest", "directory", rootPath)
	
	totalFiles := len(manifest)
	processed := 0
	skipped := 0
	
	for relativePath, fileInfo := range manifest {
		absolutePath := filepath.Join(rootPath, relativePath)
		
		// Skip if already has hash
		if fileInfo.SHA256 != "" {
			continue
		}
		
		// Skip if file should be ignored for hashing
		if he.shouldSkipHashing(absolutePath, cfg) {
			skipped++
			continue
		}
		
		hash, err := he.GenerateFileHash(absolutePath)
		if err != nil {
			he.logger.Warn("Failed to generate hash", "file", absolutePath, "error", err)
			continue
		}
		
		// Update the manifest with hash
		fileInfo.SHA256 = hash
		manifest[relativePath] = fileInfo
		
		processed++
		if processed%100 == 0 {
			he.logger.Info("Hash progress", "processed", processed, "total", totalFiles)
		}
	}
	
	he.logger.Info("Hash generation complete", 
		"processed", processed, 
		"skipped", skipped, 
		"total", totalFiles)
	
	return nil
}

// shouldSkipHashing checks if a file should be skipped for hash generation
func (he *HashEngine) shouldSkipHashing(filePath string, cfg *config.Config) bool {
	// Skip very large files (> 1GB by default)
	if info, err := os.Stat(filePath); err == nil {
		if info.Size() > 1024*1024*1024 { // 1GB
			return true
		}
	}
	
	// Skip binary files that change frequently
	skipExtensions := []string{".log", ".tmp", ".cache", ".lock", ".pid"}
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, skipExt := range skipExtensions {
		if ext == skipExt {
			return true
		}
	}
	
	return false
}

// ValidateBackupFix validates a backup and provides fixing suggestions
type ValidateBackupFix struct {
	logger logger.Logger
}

// NewValidateBackupFix creates a new validation and fix utility
func NewValidateBackupFix(log logger.Logger) *ValidateBackupFix {
	return &ValidateBackupFix{
		logger: log,
	}
}

// ValidationResult contains the results of backup validation
type ValidationResult struct {
	IsValid           bool
	TotalFiles        int
	ValidFiles        int
	InvalidFiles      int
	MissingFiles      []string
	CorruptedFiles    []string
	SizeProblems      []string
	HashProblems      []string
	PermissionIssues  []string
	Suggestions       []string
}

// ValidateBackup performs comprehensive backup validation
func (vbf *ValidateBackupFix) ValidateBackup(backupPath, sourceDir string) (*ValidationResult, error) {
	result := &ValidationResult{
		MissingFiles:     []string{},
		CorruptedFiles:   []string{},
		SizeProblems:     []string{},
		HashProblems:     []string{},
		PermissionIssues: []string{},
		Suggestions:      []string{},
	}
	
	vbf.logger.Info("Starting backup validation", "backup", backupPath, "source", sourceDir)
	
	// Load manifests
	preManifestPath := filepath.Join(backupPath, "pre_backup_manifest.json")
	preManifest, err := LoadManifest(preManifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load pre-backup manifest: %w", err)
	}
	
	postManifestPath := filepath.Join(backupPath, "post_backup_manifest.json")
	postManifest, err := LoadManifest(postManifestPath)
	if err != nil {
		vbf.logger.Warn("Post-backup manifest not found, generating one")
		// Generate post-backup manifest if it doesn't exist
		cfg := &config.Config{
			SourceDir:              sourceDir,
			DestinationDir:         filepath.Dir(backupPath),
			EnableHashVerification: true,
			IgnorePatterns:         config.DefaultIgnorePatterns(),
			AlwaysInclude:          config.DefaultAlwaysInclude(),
		}
		
		utility := NewUtility(cfg, vbf.logger)
		backupTargetDir := filepath.Join(backupPath, filepath.Base(sourceDir))
		postManifest, err = utility.GenerateFileManifest(backupTargetDir, true)
		if err != nil {
			return nil, fmt.Errorf("failed to generate post-backup manifest: %w", err)
		}
		
		// Save the generated manifest
		if err := utility.SaveManifest(postManifest, "post_backup_manifest.json"); err != nil {
			vbf.logger.Warn("Failed to save generated post-backup manifest", "error", err)
		}
	}
	
	result.TotalFiles = len(preManifest)
	
	// Validate each file
	backupTargetDir := filepath.Join(backupPath, filepath.Base(sourceDir))
	
	for relativePath, preInfo := range preManifest {
		backupFilePath := filepath.Join(backupTargetDir, relativePath)
		
		// Check if file exists in backup
		if _, err := os.Stat(backupFilePath); os.IsNotExist(err) {
			result.MissingFiles = append(result.MissingFiles, relativePath)
			result.InvalidFiles++
			continue
		}
		
		// Check file size
		if stat, err := os.Stat(backupFilePath); err == nil {
			if stat.Size() != preInfo.Size {
				result.SizeProblems = append(result.SizeProblems, 
					fmt.Sprintf("%s: expected %d bytes, got %d bytes", 
						relativePath, preInfo.Size, stat.Size()))
				result.InvalidFiles++
				continue
			}
		}
		
		// Check hash if available
		if preInfo.SHA256 != "" {
			if postInfo, exists := postManifest[relativePath]; exists {
				if postInfo.SHA256 != preInfo.SHA256 {
					result.HashProblems = append(result.HashProblems,
						fmt.Sprintf("%s: hash mismatch", relativePath))
					result.InvalidFiles++
					continue
				}
			} else {
				// Generate hash for comparison
				he := NewHashEngine(vbf.logger)
				actualHash, err := he.GenerateFileHash(backupFilePath)
				if err != nil {
					result.CorruptedFiles = append(result.CorruptedFiles,
						fmt.Sprintf("%s: cannot read file for hash verification", relativePath))
					result.InvalidFiles++
					continue
				}
				
				if actualHash != preInfo.SHA256 {
					result.HashProblems = append(result.HashProblems,
						fmt.Sprintf("%s: hash mismatch", relativePath))
					result.InvalidFiles++
					continue
				}
			}
		}
		
		result.ValidFiles++
	}
	
	// Generate suggestions based on problems found
	vbf.generateSuggestions(result)
	
	result.IsValid = result.InvalidFiles == 0
	
	vbf.logger.Info("Backup validation complete",
		"total", result.TotalFiles,
		"valid", result.ValidFiles,
		"invalid", result.InvalidFiles,
		"is_valid", result.IsValid)
	
	return result, nil
}

// generateSuggestions provides suggestions based on validation problems
func (vbf *ValidateBackupFix) generateSuggestions(result *ValidationResult) {
	if len(result.MissingFiles) > 0 {
		result.Suggestions = append(result.Suggestions,
			fmt.Sprintf("Re-run backup to restore %d missing files", len(result.MissingFiles)))
	}
	
	if len(result.HashProblems) > 0 {
		result.Suggestions = append(result.Suggestions,
			"Files with hash mismatches may be corrupted - consider re-backing up these files")
	}
	
	if len(result.SizeProblems) > 0 {
		result.Suggestions = append(result.Suggestions,
			"Files with size differences may have been truncated - verify and re-backup")
	}
	
	if len(result.PermissionIssues) > 0 {
		result.Suggestions = append(result.Suggestions,
			"Check file permissions in backup directory")
	}
	
	if result.InvalidFiles > 0 {
		result.Suggestions = append(result.Suggestions,
			"Run backup again with --verify flag to ensure integrity")
	}
}

// GenerateValidationReport creates a detailed validation report
func (vbf *ValidateBackupFix) GenerateValidationReport(result *ValidationResult, reportPath string) error {
	file, err := os.Create(reportPath)
	if err != nil {
		return fmt.Errorf("failed to create validation report: %w", err)
	}
	defer file.Close()
	
	writer := bufio.NewWriter(file)
	defer writer.Flush()
	
	fmt.Fprintf(writer, "============================================================\n")
	fmt.Fprintf(writer, "BACKUP VALIDATION REPORT\n")
	fmt.Fprintf(writer, "============================================================\n\n")
	fmt.Fprintf(writer, "Validation Date: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	
	fmt.Fprintf(writer, "SUMMARY:\n")
	fmt.Fprintf(writer, "------------------------------------------------------------\n")
	fmt.Fprintf(writer, "Total files: %d\n", result.TotalFiles)
	fmt.Fprintf(writer, "Valid files: %d\n", result.ValidFiles)
	fmt.Fprintf(writer, "Invalid files: %d\n", result.InvalidFiles)
	
	status := "✓ PASSED"
	if !result.IsValid {
		status = "⚠ FAILED"
	}
	fmt.Fprintf(writer, "Validation status: %s\n\n", status)
	
	if len(result.MissingFiles) > 0 {
		fmt.Fprintf(writer, "MISSING FILES (%d):\n", len(result.MissingFiles))
		fmt.Fprintf(writer, "------------------------------------------------------------\n")
		for _, file := range result.MissingFiles {
			fmt.Fprintf(writer, "  %s\n", file)
		}
		fmt.Fprintf(writer, "\n")
	}
	
	if len(result.HashProblems) > 0 {
		fmt.Fprintf(writer, "HASH PROBLEMS (%d):\n", len(result.HashProblems))
		fmt.Fprintf(writer, "------------------------------------------------------------\n")
		for _, problem := range result.HashProblems {
			fmt.Fprintf(writer, "  %s\n", problem)
		}
		fmt.Fprintf(writer, "\n")
	}
	
	if len(result.SizeProblems) > 0 {
		fmt.Fprintf(writer, "SIZE PROBLEMS (%d):\n", len(result.SizeProblems))
		fmt.Fprintf(writer, "------------------------------------------------------------\n")
		for _, problem := range result.SizeProblems {
			fmt.Fprintf(writer, "  %s\n", problem)
		}
		fmt.Fprintf(writer, "\n")
	}
	
	if len(result.Suggestions) > 0 {
		fmt.Fprintf(writer, "SUGGESTIONS:\n")
		fmt.Fprintf(writer, "------------------------------------------------------------\n")
		for i, suggestion := range result.Suggestions {
			fmt.Fprintf(writer, "%d. %s\n", i+1, suggestion)
		}
		fmt.Fprintf(writer, "\n")
	}
	
	vbf.logger.Info("Validation report generated", "path", reportPath)
	return nil
}