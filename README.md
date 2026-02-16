# vanilladbc-go

A Go library and CLI tool for reading and writing World of Warcraft Vanilla (1.0.0 - 1.12.3) DBC files.

## Features

- **DBD Parser** - Parse `.dbd` definition files from [VanillaDBDefs](https://github.com/suprsokr/VanillaDBDefs)
- **DBC Reader** - Read binary DBC files using DBD definitions
- **DBC Writer** - Write binary DBC files from structured data
- **CLI Tool** - Command-line interface for DBC operations

## Project Structure

```
vanilladbc-go/
├── cmd/vanilladbc/        # CLI application
├── pkg/
│   ├── dbd/               # DBD file parser
│   └── dbc/               # DBC file reader/writer
├── definitions/           # Git submodule: VanillaDBDefs
└── internal/testdata/     # Test files
```

## Installation

```bash
go get github.com/suprsokr/vanilladbc-go
```

## Usage

### As a Library

```go
package main

import (
    "github.com/suprsokr/vanilladbc-go/pkg/dbd"
    "github.com/suprsokr/vanilladbc-go/pkg/dbc"
)

func main() {
    // Parse DBD definition
    definition, err := dbd.ParseFile("definitions/Spell.dbd")
    if err != nil {
        panic(err)
    }
    
    // Read DBC file
    data, err := dbc.ReadFile("Spell.dbc", definition, "1.12.1.5875")
    if err != nil {
        panic(err)
    }
    
    // Access data
    for _, record := range data.Records {
        fmt.Println(record["ID"], record["Name"])
    }
}
```

### As a CLI Tool

```bash
# Read a DBC file
vanilladbc read Spell.dbc --build 1.12.1.5875 --output spell.json

# Write a DBC file
vanilladbc write spell.json --build 1.12.1.5875 --output Spell.dbc

# Convert DBC to JSON
vanilladbc convert Spell.dbc --build 1.12.1.5875 --format json

# Validate a DBC file
vanilladbc validate Spell.dbc --build 1.12.1.5875
```

## Development

```bash
# Clone with submodules
git clone --recursive https://github.com/suprsokr/vanilladbc-go.git

# Run tests
go test ./...

# Build CLI tool
go build -o vanilladbc ./cmd/vanilladbc
```

## Dependencies

- [VanillaDBDefs](https://github.com/suprsokr/VanillaDBDefs) - Database definitions (git submodule)

## License

See LICENSE for details.

## Credits

- [VanillaDBDefs](https://github.com/suprsokr/VanillaDBDefs) - Vanilla WoW database definitions
- [WoWDBDefs](https://github.com/wowdev/WoWDBDefs) - Original database definitions project
