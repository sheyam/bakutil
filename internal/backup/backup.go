package backup

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bakutil/internal/config"
	"bakutil/internal/types"
	"bakutil/pkg/logger"
)

// Type aliases for convenience
type FileManifest = types.FileManifest

// Utility represents the main backup utility
type Utility struct {
	config     *config.Config
	logger     logger.Logger
	stats      *types.BackupStats
	backupPath string
	logPath    string
}

// NewUtility creates a new backup utility instance
func NewUtility(cfg *config.Config, log logger.Logger) *Utility {
	return &Utility{
		config: cfg,
		logger: log,
		stats:  &types.BackupStats{},
	}
}

// SetupBackupDirectory creates a timestamped backup directory
func (u *Utility) SetupBackupDirectory() error {
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("home_backup_%s", timestamp)
	u.backupPath = filepath.Join(u.config.DestinationDir, backupName)
	
	if err := os.MkdirAll(u.backupPath, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}
	
	// Create log directory
	logDir := filepath.Join(u.backupPath, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	
	u.logPath = filepath.Join(logDir, "backup.log")
	
	u.logger.Info("Backup destination", "path", u.backupPath)
	u.logger.Info("Log file", "path", u.logPath)
	
	return nil
}

// ShouldIgnore checks if a path should be ignored based on configuration
func (u *Utility) ShouldIgnore(path string, basePath string) bool {
	// Convert to relative path for pattern matching
	relPath, err := filepath.Rel(basePath, path)
	if err != nil {
		// If we can't get relative path, use just the filename
		relPath = filepath.Base(path)
	}
	
	// Clean the path and normalize separators
	relPath = filepath.Clean(relPath)
	relPath = strings.ReplaceAll(relPath, string(filepath.Separator), "/")
	
	// Check if in always include list
	for _, include := range u.config.AlwaysInclude {
		if strings.HasPrefix(relPath, include) || strings.HasSuffix(relPath, "/"+include) {
			return false
		}
	}
	
	// Check ignore patterns
	fileName := filepath.Base(path)
	for _, pattern := range u.config.IgnorePatterns {
		if u.matchPattern(relPath, fileName, pattern) {
			return true
		}
	}
	
	return false
}

// matchPattern checks if a path matches a pattern (supports basic wildcards)
func (u *Utility) matchPattern(relPath, fileName, pattern string) bool {
	// Exact match
	if relPath == pattern || fileName == pattern {
		return true
	}
	
	// Directory match
	if strings.HasPrefix(relPath, pattern+"/") || strings.Contains(relPath, "/"+pattern+"/") {
		return true
	}
	
	// Wildcard match
	if strings.Contains(pattern, "*") {
		matched, _ := filepath.Match(pattern, fileName)
		if matched {
			return true
		}
		matched, _ = filepath.Match(pattern, relPath)
		return matched
	}
	
	// Extension match
	if strings.HasPrefix(pattern, "*.") {
		return strings.HasSuffix(fileName, pattern[1:])
	}
	
	return false
}

// CalculateFileHash calculates SHA256 hash of a file
func (u *Utility) CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// GenerateFileManifest creates a manifest of all files with their metadata and hashes
func (u *Utility) GenerateFileManifest(directory string, generateHashes bool) (types.FileManifest, error) {
	manifest := make(types.FileManifest)
	fileCount := 0
	skippedCount := 0
	
	u.logger.Info("Generating file manifest", "directory", directory, "hashes", generateHashes)
	
	err := filepath.WalkDir(directory, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) || os.IsPermission(err) {
				skippedCount++
				return nil // Skip but continue
			}
			return err
		}
		
		// Skip if should be ignored
		if u.ShouldIgnore(path, directory) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Only process files
		if d.IsDir() {
			return nil
		}
		
		// Get file info
		info, err := d.Info()
		if err != nil {
			skippedCount++
			return nil
		}
		
		// Get relative path
		relPath, err := filepath.Rel(directory, path)
		if err != nil {
			relPath = path
		}
		
		fileInfo := &types.FileInfo{
			Path:  relPath,
			Size:  info.Size(),
			MTime: info.ModTime(),
		}
		
		// Generate hash if requested
		if generateHashes {
			hash, err := u.CalculateFileHash(path)
			if err != nil {
				u.logger.Warn("Failed to calculate hash", "path", path, "error", err)
			} else {
				fileInfo.SHA256 = hash
			}
		}
		
		manifest[relPath] = fileInfo
		fileCount++
		
		// Log progress every 5000 files
		if fileCount%5000 == 0 {
			u.logger.Info("Manifest generation progress", "files", fileCount)
		}
		
		return nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}
	
	u.logger.Info("Manifest generation complete", 
		"files", fileCount, 
		"skipped", skippedCount)
	
	return manifest, nil
}

// GetBackupPath returns the current backup path
func (u *Utility) GetBackupPath() string {
	return u.backupPath
}

// GetStats returns the current backup statistics
func (u *Utility) GetStats() *types.BackupStats {
	return u.stats
}

// SaveManifest saves a file manifest to JSON file
func (u *Utility) SaveManifest(manifest types.FileManifest, filename string) error {
	manifestPath := filepath.Join(u.backupPath, filename)
	
	file, err := os.Create(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to create manifest file: %w", err)
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	
	if err := encoder.Encode(manifest); err != nil {
		return fmt.Errorf("failed to encode manifest: %w", err)
	}
	
	u.logger.Info("Manifest saved", "path", manifestPath, "files", len(manifest))
	return nil
}

// LoadManifest loads a file manifest from JSON file
func LoadManifest(manifestPath string) (types.FileManifest, error) {
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open manifest file: %w", err)
	}
	defer file.Close()
	
	var manifest types.FileManifest
	decoder := json.NewDecoder(file)
	
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}
	
	return manifest, nil
}

// ValidateBackup performs backup validation using the validation engine
func (u *Utility) ValidateBackup(backupPath string) (*types.VerificationResult, error) {
	// Use the ValidateBackupFix utility for validation
	validator := NewValidateBackupFix(u.logger)
	
	// Try to determine source directory from config or backup directory
	sourceDir := u.config.SourceDir
	if sourceDir == "" {
		// Try to infer from backup directory structure
		entries, err := os.ReadDir(backupPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read backup directory: %w", err)
		}
		
		// Look for a subdirectory that might be the backed up source
		for _, entry := range entries {
			if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
				sourceDir = filepath.Join(backupPath, entry.Name())
				break
			}
		}
		
		if sourceDir == "" {
			return nil, fmt.Errorf("could not determine source directory from backup")
		}
	}
	
	result, err := validator.ValidateBackup(backupPath, sourceDir)
	if err != nil {
		return nil, err
	}
	
	// Convert ValidationResult to VerificationResult
	issues := []string{}
	issues = append(issues, result.MissingFiles...)
	issues = append(issues, result.CorruptedFiles...)
	issues = append(issues, result.SizeProblems...)
	issues = append(issues, result.HashProblems...)
	issues = append(issues, result.PermissionIssues...)
	
	verifyResult := &types.VerificationResult{
		Success:    result.IsValid,
		TotalFiles: result.TotalFiles,
		Issues:     issues,
		ReportPath: filepath.Join(backupPath, "validation_report.txt"),
	}
	
	return verifyResult, nil
}

// AnalyzeDirectory analyzes the source directory and performs the backup
func (u *Utility) AnalyzeDirectory() error {
	u.logger.Info("Analyzing directory structure", "source", u.config.SourceDir)
	
	u.stats.StartTime = time.Now()
	
	// Generate pre-backup manifest
	u.logger.Info("Generating pre-backup manifest")
	preManifest, err := u.GenerateFileManifest(u.config.SourceDir, u.config.EnableHashVerification)
	if err != nil {
		return fmt.Errorf("failed to generate pre-backup manifest: %w", err)
	}
	
	// Save pre-backup manifest
	if err := u.SaveManifest(preManifest, "pre_backup_manifest.json"); err != nil {
		return fmt.Errorf("failed to save pre-backup manifest: %w", err)
	}
	
	// Perform the actual backup (copy files)
	targetDir := filepath.Join(u.backupPath, filepath.Base(u.config.SourceDir))
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}
	
	u.logger.Info("Copying files", "target", targetDir)
	
	filesCopied := 0
	err = filepath.WalkDir(u.config.SourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				u.stats.BrokenSymlinks++
				return nil
			}
			if os.IsPermission(err) {
				u.stats.PermissionErrors++
				return nil
			}
			return err
		}
		
		// Skip if should be ignored
		if u.ShouldIgnore(path, u.config.SourceDir) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Calculate relative path
		relPath, err := filepath.Rel(u.config.SourceDir, path)
		if err != nil {
			return err
		}
		
		destPath := filepath.Join(targetDir, relPath)
		
		if d.IsDir() {
			// Create directory
			u.stats.TotalFolders++
			return os.MkdirAll(destPath, d.Type().Perm())
		} else {
			// Copy file
			u.stats.TotalFiles++
			filesCopied++
			
			info, err := d.Info()
			if err != nil {
				return err
			}
			
			u.logger.Info("Copying file", 
				"file", relPath, 
				"size", info.Size(),
				"progress", fmt.Sprintf("%d files", u.stats.TotalFiles))
			
			u.stats.TotalSize += info.Size()
			u.stats.CopiedFiles++
			u.stats.CopiedSize += info.Size()
			
			if err := u.copyFile(path, destPath); err != nil {
				u.logger.Error("Failed to copy file", "src", path, "dst", destPath, "error", err)
				return err
			}
			
			return nil
		}
	})
	
	if err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	
	u.stats.EndTime = time.Now()
	u.logger.Info("Backup analysis completed", 
		"files", u.stats.TotalFiles,
		"directories", u.stats.TotalFolders,
		"size", u.stats.TotalSize)
	
	return nil
}

// copyFile copies a single file from source to destination
func (u *Utility) copyFile(src, dst string) error {
	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	
	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	
	// Copy contents
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}
	
	// Copy file permissions and timestamps
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}
	
	if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
		return err
	}
	
	return os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
}

// performPostBackupVerification performs verification immediately after backup
func (u *Utility) performPostBackupVerification() error {
	targetDir := filepath.Join(u.backupPath, filepath.Base(u.config.SourceDir))
	
	u.logger.Info("Generating post-backup manifest for verification")
	postManifest, err := u.GenerateFileManifest(targetDir, u.config.EnableHashVerification)
	if err != nil {
		return fmt.Errorf("failed to generate post-backup manifest: %w", err)
	}
	
	// Save post-backup manifest
	if err := u.SaveManifest(postManifest, "post_backup_manifest.json"); err != nil {
		return fmt.Errorf("failed to save post-backup manifest: %w", err)
	}
	
	// Load pre-backup manifest for comparison
	preManifestPath := filepath.Join(u.backupPath, "pre_backup_manifest.json")
	preManifest, err := LoadManifest(preManifestPath)
	if err != nil {
		return fmt.Errorf("failed to load pre-backup manifest: %w", err)
	}
	
	// Compare manifests and log results
	ve := NewVerificationEngine(u)
	comparison := ve.CompareManifests(preManifest, postManifest)
	
	u.logger.Info("Backup verification results",
		"identical_files", comparison.IdenticalFiles,
		"missing_files", len(comparison.MissingFiles),
		"modified_files", len(comparison.ModifiedFiles),
		"new_files", len(comparison.NewFiles))
	
	// Log any issues found
	for _, missing := range comparison.MissingFiles {
		u.logger.Warn("File missing from backup", "file", missing)
	}
	
	for _, modified := range comparison.ModifiedFiles {
		u.logger.Warn("File hash mismatch", 
			"file", modified.Path,
			"reason", modified.Reason,
			"original_hash", modified.OldHash,
			"backup_hash", modified.NewHash)
	}
	
	// Generate verification report
	if err := ve.GenerateVerificationReport(preManifest, postManifest, comparison); err != nil {
		return fmt.Errorf("failed to generate verification report: %w", err)
	}
	
	return nil
}

// generateBackupSummary generates a comprehensive backup summary report
func (u *Utility) generateBackupSummary() error {
	summaryPath := filepath.Join(u.backupPath, "backup_summary.txt")
	
	file, err := os.Create(summaryPath)
	if err != nil {
		return fmt.Errorf("failed to create summary file: %w", err)
	}
	defer file.Close()
	
	duration := u.stats.EndTime.Sub(u.stats.StartTime)
	
	fmt.Fprintf(file, "============================================================\n")
	fmt.Fprintf(file, "BACKUP SUMMARY REPORT\n")
	fmt.Fprintf(file, "============================================================\n\n")
	fmt.Fprintf(file, "Backup Date: %s\n", u.stats.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "Duration: %v\n\n", duration)
	
	fmt.Fprintf(file, "SOURCE:\n")
	fmt.Fprintf(file, "Source Directory: %s\n", u.config.SourceDir)
	fmt.Fprintf(file, "Destination: %s\n\n", u.backupPath)
	
	fmt.Fprintf(file, "STATISTICS:\n")
	fmt.Fprintf(file, "Total Files: %d\n", u.stats.TotalFiles)
	fmt.Fprintf(file, "Total Directories: %d\n", u.stats.TotalFolders)
	fmt.Fprintf(file, "Total Size: %d bytes (%.2f MB)\n", u.stats.TotalSize, float64(u.stats.TotalSize)/1024/1024)
	fmt.Fprintf(file, "Files Copied: %d\n", u.stats.CopiedFiles)
	fmt.Fprintf(file, "Size Copied: %d bytes (%.2f MB)\n", u.stats.CopiedSize, float64(u.stats.CopiedSize)/1024/1024)
	
	if u.stats.BrokenSymlinks > 0 {
		fmt.Fprintf(file, "Broken Symlinks: %d\n", u.stats.BrokenSymlinks)
	}
	if u.stats.PermissionErrors > 0 {
		fmt.Fprintf(file, "Permission Errors: %d\n", u.stats.PermissionErrors)
	}
	
	fmt.Fprintf(file, "\nCONFIGURATION:\n")
	fmt.Fprintf(file, "Hash Verification: %v\n", u.config.EnableHashVerification)
	fmt.Fprintf(file, "Ignore Patterns: %v\n", u.config.IgnorePatterns)
	
	if duration > 0 {
		throughput := float64(u.stats.CopiedSize) / duration.Seconds()
		fmt.Fprintf(file, "\nPERFORMANCE:\n")
		fmt.Fprintf(file, "Throughput: %.2f MB/s\n", throughput/1024/1024)
		fmt.Fprintf(file, "Files per second: %.2f\n", float64(u.stats.CopiedFiles)/duration.Seconds())
	}
	
	u.logger.Info("Backup summary generated", "path", summaryPath)
	return nil
}

// PerformBackup performs the complete backup process
func (u *Utility) PerformBackup() error {
	// Setup backup directory
	if err := u.SetupBackupDirectory(); err != nil {
		return fmt.Errorf("failed to setup backup directory: %w", err)
	}
	
	// Analyze directory and perform backup
	if err := u.AnalyzeDirectory(); err != nil {
		return fmt.Errorf("failed to analyze directory: %w", err)
	}
	
	// Perform post-backup verification if hashes are enabled
	if u.config.EnableHashVerification {
		u.logger.Info("Starting post-backup verification")
		if err := u.performPostBackupVerification(); err != nil {
			u.logger.Warn("Post-backup verification encountered issues", "error", err)
			// Don't fail the backup, just warn
		} else {
			u.logger.Info("Post-backup verification completed successfully")
		}
	}
	
	// Save configuration
	configPath := filepath.Join(u.backupPath, "backup_config.json")
	configFile, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer configFile.Close()
	
	encoder := json.NewEncoder(configFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(u.config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	
	// Generate backup summary report
	if err := u.generateBackupSummary(); err != nil {
		u.logger.Warn("Failed to generate backup summary", "error", err)
	}
	
	return nil
}