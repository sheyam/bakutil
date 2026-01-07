package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"bakutil/internal/backup"
)

var (
	sourceDir         string
	generateHashes    bool
)

var verifyCmd = &cobra.Command{
	Use:   "verify [backup_directory]",
	Short: "Verify the integrity of a backup",
	Long: `Verify backup integrity by comparing manifests and optionally checking file hashes.
	
This command can complete verification for backups that were interrupted or perform
a full verification check on completed backups.

Examples:
  bakutil verify /backups/home_backup_20240101_120000 --source /home/user
  bakutil verify ./backup --source . --hashes`,
	Args: cobra.ExactArgs(1),
	RunE: runVerify,
}

var validateCmd = &cobra.Command{
	Use:   "validate [backup_directory]",
	Short: "Validate backup and suggest fixes",
	Long: `Validate backup integrity and provide suggestions for fixing any issues found.
	
This command performs comprehensive validation including file existence, size checks,
hash verification (if available), and permission analysis.

Examples:
  bakutil validate /backups/home_backup_20240101_120000 --source /home/user`,
	Args: cobra.ExactArgs(1),
	RunE: runValidate,
}

func init() {
	// Verify command
	verifyCmd.Flags().StringVarP(&sourceDir, "source", "s", "", "Source directory that was backed up (required)")
	verifyCmd.Flags().BoolVar(&generateHashes, "hashes", false, "Generate hashes for verification")
	if err := verifyCmd.MarkFlagRequired("source"); err != nil {
		panic(err)
	}
	
	// Validate command  
	validateCmd.Flags().StringVarP(&sourceDir, "source", "s", "", "Source directory that was backed up (required)")
	if err := validateCmd.MarkFlagRequired("source"); err != nil {
		panic(err)
	}
	
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(validateCmd)
}

func runVerify(cmd *cobra.Command, args []string) error {
	backupPath := args[0]
	
	// Convert to absolute paths
	backupPath, err := filepath.Abs(backupPath)
	if err != nil {
		return fmt.Errorf("failed to resolve backup path: %w", err)
	}
	
	sourceDir, err := filepath.Abs(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to resolve source path: %w", err)
	}
	
	// Check if backup directory exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup directory does not exist: %s", backupPath)
	}
	
	fmt.Printf("🔍 Verifying backup integrity...\n")
	fmt.Printf("   Backup: %s\n", backupPath)
	fmt.Printf("   Source: %s\n", sourceDir)
	fmt.Printf("   Generate hashes: %v\n\n", generateHashes)
	
	// Perform verification
	result, err := backup.CompleteBackupVerification(backupPath, sourceDir, generateHashes)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}
	
	// Display results
	if result.Success {
		fmt.Printf("✅ Verification PASSED!\n")
		fmt.Printf("   Total files verified: %d\n", result.TotalFiles)
	} else {
		fmt.Printf("⚠️  Verification FAILED!\n")
		fmt.Printf("   Total files: %d\n", result.TotalFiles)
		if result.Comparison != nil {
			fmt.Printf("   Missing files: %d\n", len(result.Comparison.MissingFiles))
			fmt.Printf("   Modified files: %d\n", len(result.Comparison.ModifiedFiles))
		}
		
		if result.ReportPath != "" {
			fmt.Printf("   📄 Detailed report: %s\n", result.ReportPath)
		}
	}
	
	return nil
}

func runValidate(cmd *cobra.Command, args []string) error {
	backupPath := args[0]
	
	// Convert to absolute paths
	backupPath, err := filepath.Abs(backupPath)
	if err != nil {
		return fmt.Errorf("failed to resolve backup path: %w", err)
	}
	
	sourceDir, err := filepath.Abs(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to resolve source path: %w", err)
	}
	
	// Check if backup directory exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup directory does not exist: %s", backupPath)
	}
	
	fmt.Printf("🔍 Validating backup and analyzing issues...\n")
	fmt.Printf("   Backup: %s\n", backupPath)
	fmt.Printf("   Source: %s\n\n", sourceDir)
	
	log := getLogger()
	validator := backup.NewValidateBackupFix(log)
	
	// Perform validation
	result, err := validator.ValidateBackup(backupPath, sourceDir)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	
	// Display results
	fmt.Printf("📊 Validation Results:\n")
	fmt.Printf("   Total files: %d\n", result.TotalFiles)
	fmt.Printf("   Valid files: %d\n", result.ValidFiles)
	fmt.Printf("   Invalid files: %d\n", result.InvalidFiles)
	
	if result.IsValid {
		fmt.Printf("   Status: ✅ VALID\n\n")
	} else {
		fmt.Printf("   Status: ⚠️  INVALID\n\n")
		
		// Show specific issues
		if len(result.MissingFiles) > 0 {
			fmt.Printf("Missing Files (%d):\n", len(result.MissingFiles))
			for i, file := range result.MissingFiles {
				if i < 10 { // Show first 10
					fmt.Printf("   - %s\n", file)
				} else {
					fmt.Printf("   ... and %d more\n", len(result.MissingFiles)-i)
					break
				}
			}
			fmt.Println()
		}
		
		if len(result.HashProblems) > 0 {
			fmt.Printf("Hash Problems (%d):\n", len(result.HashProblems))
			for i, problem := range result.HashProblems {
				if i < 10 { // Show first 10
					fmt.Printf("   - %s\n", problem)
				} else {
					fmt.Printf("   ... and %d more\n", len(result.HashProblems)-i)
					break
				}
			}
			fmt.Println()
		}
		
		if len(result.SizeProblems) > 0 {
			fmt.Printf("Size Problems (%d):\n", len(result.SizeProblems))
			for i, problem := range result.SizeProblems {
				if i < 10 { // Show first 10
					fmt.Printf("   - %s\n", problem)
				} else {
					fmt.Printf("   ... and %d more\n", len(result.SizeProblems)-i)
					break
				}
			}
			fmt.Println()
		}
		
		// Show suggestions
		if len(result.Suggestions) > 0 {
			fmt.Printf("💡 Suggestions:\n")
			for i, suggestion := range result.Suggestions {
				fmt.Printf("   %d. %s\n", i+1, suggestion)
			}
			fmt.Println()
		}
	}
	
	// Generate detailed report
	reportPath := filepath.Join(backupPath, "validation_report.txt")
	if err := validator.GenerateValidationReport(result, reportPath); err != nil {
		fmt.Printf("⚠️  Failed to generate detailed report: %v\n", err)
	} else {
		fmt.Printf("📄 Detailed report saved: %s\n", reportPath)
	}
	
	return nil
}