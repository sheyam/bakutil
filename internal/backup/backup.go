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