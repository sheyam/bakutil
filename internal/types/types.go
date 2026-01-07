package types

import "time"

// FileInfo represents metadata about a file
type FileInfo struct {
	Path   string    `json:"path"`
	Size   int64     `json:"size"`
	MTime  time.Time `json:"mtime"`
	SHA256 string    `json:"sha256,omitempty"`
}

// FileManifest maps file paths to their metadata
type FileManifest map[string]*FileInfo

// BackupStats contains statistics about a backup operation
type BackupStats struct {
	StartTime        time.Time         `json:"start_time"`
	EndTime          time.Time         `json:"end_time"`
	Duration         time.Duration     `json:"duration"`
	TotalItems       int64             `json:"total_items"`
	TotalFiles       int64             `json:"total_files"`
	TotalFolders     int64             `json:"total_folders"`
	TotalSize        int64             `json:"total_size"`
	IgnoredItems     int64             `json:"ignored_items"`
	IgnoredFiles     int64             `json:"ignored_files"`
	IgnoredFolders   int64             `json:"ignored_folders"`
	IgnoredSize      int64             `json:"ignored_size"`
	CopiedFiles      int64             `json:"copied_files"`
	CopiedSize       int64             `json:"copied_size"`
	SkippedFiles     int64             `json:"skipped_files"`
	FailedFiles      int64             `json:"failed_files"`
	BrokenSymlinks   int64             `json:"broken_symlinks"`
	PermissionErrors int64             `json:"permission_errors"`
	LargeFiles       []LargeFileInfo   `json:"large_files"`
}

// LargeFileInfo contains information about large files
type LargeFileInfo struct {
	Path   string  `json:"path"`
	SizeMB float64 `json:"size_mb"`
}

// BackupComparison contains results of comparing two manifests
type BackupComparison struct {
	TotalFilesPre   int                `json:"total_files_pre"`
	TotalFilesPost  int                `json:"total_files_post"`
	IdenticalFiles  int                `json:"identical_files"`
	MissingFiles    []string           `json:"missing_files"`
	NewFiles        []string           `json:"new_files"`
	ModifiedFiles   []ModifiedFile     `json:"modified_files"`
	SizeDifferences []SizeDifference   `json:"size_differences"`
}

// ModifiedFile represents a file that has been modified
type ModifiedFile struct {
	Path     string `json:"path"`
	Reason   string `json:"reason"`
	OldHash  string `json:"old_hash,omitempty"`
	NewHash  string `json:"new_hash,omitempty"`
	OldSize  int64  `json:"old_size,omitempty"`
	NewSize  int64  `json:"new_size,omitempty"`
	OldMTime string `json:"old_mtime,omitempty"`
	NewMTime string `json:"new_mtime,omitempty"`
}

// SizeDifference represents a file with different sizes
type SizeDifference struct {
	Path    string `json:"path"`
	OldSize int64  `json:"old_size"`
	NewSize int64  `json:"new_size"`
}

// VerificationResult contains the result of backup verification
type VerificationResult struct {
	Success       bool              `json:"success"`
	TotalFiles    int               `json:"total_files"`
	VerifiedFiles int               `json:"verified_files"`
	Issues        []string          `json:"issues"`
	Comparison    *BackupComparison `json:"comparison"`
	ReportPath    string            `json:"report_path"`
}