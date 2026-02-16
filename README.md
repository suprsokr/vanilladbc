# vanilladbc

A Go library for reading and writing World of Warcraft Vanilla (1.0.0 - 1.12.3) DBC files.

This is a **library-only** package. For CLI tools and format conversion plugins, see:
- [vanilladbc-cli](https://github.com/suprsokr/vanilladbc-cli) - Command-line tool
- [vanilladbc-json](https://github.com/suprsokr/vanilladbc-json) - JSON conversion plugin

## Features

- **DBD Parser** - Parse `.dbd` definition files from [VanillaDBDefs](https://github.com/suprsokr/VanillaDBDefs)
- **DBC Reader** - Read binary DBC files using DBD definitions
- **DBC Writer** - Write binary DBC files from structured data
- **Streaming API** - Memory-efficient streaming for large files
- **Iterator API** - Simple iteration over DBC records
- **Plugin Interface** - Standard interface for format conversion plugins

## Project Structure

```
vanilladbc/
├── pkg/
│   ├── dbd/               # DBD file parser
│   ├── dbc/               # DBC file reader/writer/streaming
│   └── plugin/            # Plugin interfaces
├── definitions/           # Git submodule: VanillaDBDefs
└── internal/testdata/     # Test files
```

## Installation

```bash
go get github.com/suprsokr/vanilladbc
```

## Usage

### Basic Reading

```go
package main

import (
    "fmt"
    "github.com/suprsokr/vanilladbc/pkg/dbd"
    "github.com/suprsokr/vanilladbc/pkg/dbc"
)

func main() {
    // Parse DBD definition
    definition, err := dbd.ParseFile("definitions/Spell.dbd")
    if err != nil {
        panic(err)
    }
    
    // Read entire DBC file into memory
    data, err := dbc.ReadFile("Spell.dbc", definition, "1.12.1.5875")
    if err != nil {
        panic(err)
    }
    
    // Access all records
    for _, record := range data.Records {
        fmt.Println(record["ID"], record["Name"])
    }
}
```

### Streaming (Memory Efficient)

```go
// Stream records one at a time (better for large files)
err := dbc.StreamFile("Spell.dbc", definition, "1.12.1.5875", 
    func(recordNum int, record dbc.Record) error {
        fmt.Printf("Record %d: ID=%v Name=%v\n", 
            recordNum, record["ID"], record["Name"])
        return nil
    })
```

### Iterator Pattern

```go
file, _ := os.Open("Spell.dbc")
defer file.Close()

build, _ := dbd.NewBuild("1.12.1.5875")
iter, err := dbc.NewIterator(file, definition, *build)
if err != nil {
    panic(err)
}

for iter.Next() {
    record := iter.Record()
    fmt.Printf("Record %d/%d: %v\n", iter.Index(), iter.Count(), record["ID"])
}

if iter.Err() != nil {
    panic(iter.Err())
}
```

### Writing DBC Files

```go
// Create records
records := []dbc.Record{
    {"ID": uint32(1), "Name": "Fireball", "School": uint32(2)},
    {"ID": uint32(2), "Name": "Frostbolt", "School": uint32(4)},
}

// Get version definition
build, _ := dbd.NewBuild("1.12.1.5875")
versionDef, _ := definition.GetVersionDefinition(*build)

// Write DBC file
dbcFile := &dbc.DBCFile{
    Records:    records,
    Definition: versionDef,
    Columns:    definition.Columns,
}

err := dbc.WriteFile("output.dbc", dbcFile)
```

### Implementing a Plugin

```go
import "github.com/suprsokr/vanilladbc/pkg/plugin"

type MyPlugin struct {
    // your fields
}

func (p *MyPlugin) WriteHeader(versionDef *dbd.VersionDefinition, 
                                columns map[string]dbd.ColumnDefinition) error {
    // Setup output format
    return nil
}

func (p *MyPlugin) WriteRecord(record dbc.Record) error {
    // Convert and write record
    return nil
}

func (p *MyPlugin) WriteFooter() error {
    // Finalize output
    return nil
}
```

## Development

```bash
# Clone with submodules
git clone --recursive https://github.com/suprsokr/vanilladbc.git

# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...
```

## Dependencies

- [VanillaDBDefs](https://github.com/suprsokr/VanillaDBDefs) - Database definitions (git submodule)

## License

See LICENSE for details.

## Credits

- [VanillaDBDefs](https://github.com/suprsokr/VanillaDBDefs) - Vanilla WoW database definitions
- [WoWDBDefs](https://github.com/wowdev/WoWDBDefs) - Original database definitions project
