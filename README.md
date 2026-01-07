# bakutil

A production-ready backup utility written in Go with intelligent file management and verification capabilities.

## Features

- 🔄 **Intelligent Exclusions**: Automatically excludes cache directories, build artifacts, and unnecessary files
- ⚡ **High Performance**: Uses rsync when available, Go native copy as fallback
- 🔍 **Hash Verification**: SHA256 verification for data integrity
- 📝 **Smart Patterns**: Configurable ignore patterns and always-include rules
- 🖥️ **Cross-Platform**: Works on macOS, Linux, and Windows
- 📊 **Detailed Reporting**: Comprehensive backup reports and statistics
- 🎯 **CLI Interface**: Professional command-line interface with help
- 🔧 **Flexible Configuration**: YAML-based configuration with sensible defaults

## Quick Start

### Installation

#### Download Pre-built Binary
Download the latest release for your platform from the [Releases page](https://github.com/sheyam/bakutil/releases).

#### Build from Source
```bash
git clone https://github.com/sheyam/bakutil.git
cd bakutil
make build
```

### Basic Usage

1. **Run a backup**:
```bash
./backup-util backup --destination /path/to/backup
```

2. **Get help**:
```bash
./backup-util --help
./backup-util backup --help
```

## Configuration

The utility uses sensible defaults but can be customized with a `backup.yaml` file:

```yaml
# Source directory (defaults to user home)
source_dir: "/Users/yourusername"

# Backup settings
use_rsync: true
enable_hash_verification: true
large_file_threshold_mb: 100

# File patterns to ignore
ignore_patterns:
  - "*.log"
  - "*.tmp"
  - ".cache"
  - "node_modules"
  - "__pycache__"
  - "Library/Caches"

# Always include these patterns
always_include:
  - "Documents"
  - "Desktop"
  - ".ssh"
  - ".gitconfig"
```

## Commands

- `backup` - Perform backup operation
- `verify` - Verify existing backup integrity
- `analyze` - Analyze directory without backing up
- `config` - Manage configuration

## Development

```bash
# Install dependencies
go mod download

# Run tests
make test

# Build
make build

# Build for all platforms
make build-all
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `make test lint`
6. Submit a pull request

## License

[Add your license here]
