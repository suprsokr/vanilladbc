package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/suprsokr/vanilladbc-go/pkg/dbc"
	"github.com/suprsokr/vanilladbc-go/pkg/dbd"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "read":
		if err := cmdRead(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "write":
		if err := cmdWrite(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "info":
		if err := cmdInfo(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("vanilladbc - Vanilla WoW DBC file reader/writer")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  vanilladbc read <dbc_file> <dbd_file> <build> [output.json]")
	fmt.Println("  vanilladbc write <json_file> <dbd_file> <build> <output.dbc>")
	fmt.Println("  vanilladbc info <dbd_file> <build>")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  vanilladbc read Spell.dbc definitions/Spell.dbd 1.12.1.5875 spell.json")
	fmt.Println("  vanilladbc write spell.json definitions/Spell.dbd 1.12.1.5875 Spell.dbc")
	fmt.Println("  vanilladbc info definitions/Spell.dbd 1.12.1.5875")
}

func cmdRead() error {
	if len(os.Args) < 5 {
		return fmt.Errorf("usage: vanilladbc read <dbc_file> <dbd_file> <build> [output.json]")
	}

	dbcFile := os.Args[2]
	dbdFile := os.Args[3]
	buildStr := os.Args[4]
	outputFile := ""
	if len(os.Args) > 5 {
		outputFile = os.Args[5]
	}

	// Parse DBD file
	fmt.Printf("Parsing DBD file: %s\n", dbdFile)
	dbdDef, err := dbd.ParseFile(dbdFile)
	if err != nil {
		return fmt.Errorf("failed to parse DBD file: %w", err)
	}

	// Read DBC file
	fmt.Printf("Reading DBC file: %s (build %s)\n", dbcFile, buildStr)
	dbcData, err := dbc.ReadFile(dbcFile, dbdDef, buildStr)
	if err != nil {
		return fmt.Errorf("failed to read DBC file: %w", err)
	}

	fmt.Printf("Successfully read %d records\n", len(dbcData.Records))

	// Convert to JSON
	jsonData, err := json.MarshalIndent(dbcData.Records, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Output
	if outputFile != "" {
		if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Wrote JSON output to: %s\n", outputFile)
	} else {
		fmt.Println(string(jsonData))
	}

	return nil
}

func cmdWrite() error {
	if len(os.Args) < 6 {
		return fmt.Errorf("usage: vanilladbc write <json_file> <dbd_file> <build> <output.dbc>")
	}

	jsonFile := os.Args[2]
	dbdFile := os.Args[3]
	buildStr := os.Args[4]
	outputFile := os.Args[5]

	// Parse DBD file
	fmt.Printf("Parsing DBD file: %s\n", dbdFile)
	dbdDef, err := dbd.ParseFile(dbdFile)
	if err != nil {
		return fmt.Errorf("failed to parse DBD file: %w", err)
	}

	// Get version definition
	build, err := dbd.NewBuild(buildStr)
	if err != nil {
		return fmt.Errorf("invalid build: %w", err)
	}

	versionDef, err := dbdDef.GetVersionDefinition(*build)
	if err != nil {
		return fmt.Errorf("failed to get version definition: %w", err)
	}

	// Read JSON file
	fmt.Printf("Reading JSON file: %s\n", jsonFile)
	jsonData, err := os.ReadFile(jsonFile)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}

	var records []dbc.Record
	if err := json.Unmarshal(jsonData, &records); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Create DBC structure
	dbcData := &dbc.DBCFile{
		Records:    records,
		Definition: versionDef,
		Columns:    dbdDef.Columns,
	}

	// Write DBC file
	fmt.Printf("Writing DBC file: %s\n", outputFile)
	if err := dbc.WriteFile(outputFile, dbcData); err != nil {
		return fmt.Errorf("failed to write DBC file: %w", err)
	}

	fmt.Printf("Successfully wrote %d records to %s\n", len(records), outputFile)

	return nil
}

func cmdInfo() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: vanilladbc info <dbd_file> <build>")
	}

	dbdFile := os.Args[2]
	buildStr := os.Args[3]

	// Parse DBD file
	fmt.Printf("Parsing DBD file: %s\n", dbdFile)
	dbdDef, err := dbd.ParseFile(dbdFile)
	if err != nil {
		return fmt.Errorf("failed to parse DBD file: %w", err)
	}

	// Get version definition
	build, err := dbd.NewBuild(buildStr)
	if err != nil {
		return fmt.Errorf("invalid build: %w", err)
	}

	versionDef, err := dbdDef.GetVersionDefinition(*build)
	if err != nil {
		return fmt.Errorf("failed to get version definition: %w", err)
	}

	// Print info
	tableName := filepath.Base(dbdFile)
	tableName = tableName[:len(tableName)-4] // Remove .dbd extension

	fmt.Println()
	fmt.Printf("Table: %s\n", tableName)
	fmt.Printf("Build: %s\n", buildStr)
	fmt.Println()
	fmt.Printf("Total Columns Defined: %d\n", len(dbdDef.Columns))
	fmt.Printf("Fields in Build: %d\n", len(versionDef.Definitions))
	fmt.Println()
	fmt.Println("Field Definitions:")
	fmt.Println("------------------")

	for i, def := range versionDef.Definitions {
		colDef := dbdDef.Columns[def.Column]
		typeStr := string(colDef.Type)
		
		if colDef.Type == dbd.TypeInt || colDef.Type == dbd.TypeUInt {
			signStr := ""
			if def.IsUnsigned {
				signStr = "unsigned "
			}
			typeStr = fmt.Sprintf("%s%s<%d>", signStr, typeStr, def.Size)
		}
		
		arrayStr := ""
		if def.ArraySize > 0 {
			arrayStr = fmt.Sprintf("[%d]", def.ArraySize)
		}
		
		idStr := ""
		if def.IsID {
			idStr = " (ID)"
		}
		
		fkStr := ""
		if colDef.ForeignTable != "" {
			fkStr = fmt.Sprintf(" -> %s::%s", colDef.ForeignTable, colDef.ForeignColumn)
		}

		fmt.Printf("%3d. %-30s %s%s%s%s\n", i+1, def.Column, typeStr, arrayStr, idStr, fkStr)
	}

	return nil
}
