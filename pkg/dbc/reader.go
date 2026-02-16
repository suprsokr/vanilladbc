package dbc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/suprsokr/vanilladbc-go/pkg/dbd"
)

// ReadFile reads and parses a DBC file from the given path
func ReadFile(path string, dbdDef *dbd.DBDefinition, buildVersion string) (*DBCFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	build, err := dbd.NewBuild(buildVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid build version: %w", err)
	}

	return Read(file, dbdDef, *build)
}

// Read reads and parses a DBC file from an io.Reader
func Read(r io.Reader, dbdDef *dbd.DBDefinition, build dbd.Build) (*DBCFile, error) {
	// Read entire file into memory
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	if len(data) < 20 {
		return nil, fmt.Errorf("file too small to be a valid DBC file")
	}

	// Parse header
	var header Header
	buf := bytes.NewReader(data)

	if err := binary.Read(buf, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Verify magic
	if string(header.Magic[:]) != "WDBC" {
		return nil, fmt.Errorf("invalid magic: expected WDBC, got %s", string(header.Magic[:]))
	}

	// Get version definition for this build
	versionDef, err := dbdDef.GetVersionDefinition(build)
	if err != nil {
		return nil, fmt.Errorf("failed to get version definition: %w", err)
	}

	dbc := &DBCFile{
		Header:     header,
		Records:    make([]Record, header.RecordCount),
		Definition: versionDef,
		Columns:    dbdDef.Columns,
	}

	// Calculate string table offset
	stringTableOffset := 20 + (header.RecordCount * header.RecordSize)
	if uint32(len(data)) < stringTableOffset+header.StringTableSize {
		return nil, fmt.Errorf("file too small for declared string table size")
	}

	// Read string table
	dbc.StringTable = data[stringTableOffset : stringTableOffset+header.StringTableSize]

	// Read records
	recordData := data[20:stringTableOffset]
	recordBuf := bytes.NewReader(recordData)

	for i := uint32(0); i < header.RecordCount; i++ {
		record, err := readRecord(recordBuf, versionDef, dbdDef.Columns, dbc.StringTable)
		if err != nil {
			return nil, fmt.Errorf("failed to read record %d: %w", i, err)
		}
		dbc.Records[i] = record
	}

	return dbc, nil
}

// readRecord reads a single record from the DBC file
func readRecord(r io.Reader, versionDef *dbd.VersionDefinition, columns map[string]dbd.ColumnDefinition, stringTable []byte) (Record, error) {
	record := make(Record)

	for _, def := range versionDef.Definitions {
		colDef, ok := columns[def.Column]
		if !ok {
			return nil, fmt.Errorf("column %s not found in column definitions", def.Column)
		}

		arraySize := def.ArraySize
		if arraySize == 0 {
			arraySize = 1 // Non-array field
		}

		// Read array or single value
		if arraySize > 1 {
			arr := make([]interface{}, arraySize)
			for i := 0; i < arraySize; i++ {
				val, err := readValue(r, colDef.Type, def.Size, def.IsUnsigned, stringTable)
				if err != nil {
					return nil, fmt.Errorf("failed to read array element %d of %s: %w", i, def.Column, err)
				}
				arr[i] = val
			}
			record[def.Column] = arr
		} else {
			val, err := readValue(r, colDef.Type, def.Size, def.IsUnsigned, stringTable)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", def.Column, err)
			}
			record[def.Column] = val
		}
	}

	return record, nil
}

// readValue reads a single value based on type and size
func readValue(r io.Reader, colType dbd.ColumnType, size int, isUnsigned bool, stringTable []byte) (interface{}, error) {
	switch colType {
	case dbd.TypeInt, dbd.TypeUInt:
		return readInteger(r, size, isUnsigned)
	case dbd.TypeFloat:
		var val float32
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return nil, err
		}
		return val, nil
	case dbd.TypeString:
		var offset uint32
		if err := binary.Read(r, binary.LittleEndian, &offset); err != nil {
			return nil, err
		}
		return readStringFromTable(stringTable, offset)
	case dbd.TypeLocString:
		// LocString is 16 strings (one per locale) + flags
		// In vanilla, it's 16 string offsets (uint32) followed by a flags uint32
		var locString LocString
		for i := 0; i < 16; i++ {
			var offset uint32
			if err := binary.Read(r, binary.LittleEndian, &offset); err != nil {
				return nil, err
			}
			str, err := readStringFromTable(stringTable, offset)
			if err != nil {
				return nil, err
			}
			locString[i] = str
		}
		// Read flags (not used in vanilla typically)
		var flags uint32
		if err := binary.Read(r, binary.LittleEndian, &flags); err != nil {
			return nil, err
		}
		return locString, nil
	default:
		return nil, fmt.Errorf("unsupported column type: %s", colType)
	}
}

// readInteger reads an integer of the specified size and signedness
func readInteger(r io.Reader, size int, isUnsigned bool) (interface{}, error) {
	switch size {
	case 8:
		if isUnsigned {
			var val uint8
			if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
				return nil, err
			}
			return val, nil
		}
		var val int8
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return nil, err
		}
		return val, nil
	case 16:
		if isUnsigned {
			var val uint16
			if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
				return nil, err
			}
			return val, nil
		}
		var val int16
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return nil, err
		}
		return val, nil
	case 32:
		if isUnsigned {
			var val uint32
			if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
				return nil, err
			}
			return val, nil
		}
		var val int32
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return nil, err
		}
		return val, nil
	case 64:
		if isUnsigned {
			var val uint64
			if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
				return nil, err
			}
			return val, nil
		}
		var val int64
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return nil, err
		}
		return val, nil
	default:
		return nil, fmt.Errorf("unsupported integer size: %d", size)
	}
}

// readStringFromTable reads a null-terminated string from the string table at the given offset
func readStringFromTable(stringTable []byte, offset uint32) (string, error) {
	if offset == 0 {
		return "", nil
	}

	if int(offset) >= len(stringTable) {
		return "", fmt.Errorf("string offset %d out of bounds (table size: %d)", offset, len(stringTable))
	}

	// Find null terminator
	end := int(offset)
	for end < len(stringTable) && stringTable[end] != 0 {
		end++
	}

	return string(stringTable[offset:end]), nil
}
