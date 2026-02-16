# Contributing to vanilladbc-go

Thank you for your interest in contributing to vanilladbc-go!

## Development Setup

1. Clone the repository with submodules:
```bash
git clone --recursive https://github.com/suprsokr/vanilladbc-go.git
cd vanilladbc-go
```

2. Install Go 1.21 or later

3. Run tests:
```bash
go test ./...
```

## Project Structure

```
vanilladbc-go/
├── cmd/vanilladbc/      # CLI application
├── pkg/
│   ├── dbd/             # DBD file parser
│   │   ├── types.go     # Type definitions
│   │   ├── parser.go    # Parser implementation
│   │   └── parser_test.go
│   └── dbc/             # DBC file reader/writer
│       ├── types.go     # Type definitions
│       ├── reader.go    # DBC reader
│       └── writer.go    # DBC writer
├── definitions/         # Git submodule: VanillaDBDefs
└── internal/testdata/   # Test files
```

## Guidelines

### Code Style

- Follow standard Go conventions
- Run `go fmt` before committing
- Write tests for new features
- Document public APIs

### Testing

- Add tests for new functionality
- Ensure all tests pass: `go test ./...`
- Test with real vanilla DBC files when possible

### Pull Requests

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run tests and linters
6. Submit a pull request

### Commit Messages

- Use clear, descriptive commit messages
- Reference issues when applicable
- Keep commits focused and atomic

## Reporting Issues

When reporting issues, please include:
- Go version
- Operating system
- Steps to reproduce
- Expected vs actual behavior
- Sample files if applicable (for DBC/DBD issues)

## Questions?

Feel free to open an issue for questions or clarifications.
