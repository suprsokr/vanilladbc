package dbc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/suprsokr/vanilladbc-go/pkg/dbd"
)

// WriteFile writes a DBC file to the given path
func WriteFile(path string, dbc *DBCFile) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	return Write(file, dbc)
}

// Write writes a DBC file to an io.Writer
func Write(w io.Writer, dbc *DBCFile) error {
	// Build string table
	stringTable, stringOffsets := buildStringTable(dbc)

	// Update header with actual values
	dbc.Header.Magic = [4]byte{'W', 'D', 'B', 'C'}
	dbc.Header.RecordCount = uint32(len(dbc.Records))
	dbc.Header.StringTableSize = uint32(len(stringTable))

	// Calculate record size and field count
	recordSize, fieldCount := calculateRecordMetrics(dbc.Definition, dbc.Columns)
	dbc.Header.RecordSize = recordSize
	dbc.Header.FieldCount = fieldCount

	// Write header
	if err := binary.Write(w, binary.LittleEndian, &dbc.Header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write records
	for i, record := range dbc.Records {
		if err := writeRecord(w, record, dbc.Definition, dbc.Columns, stringOffsets); err != nil {
			return fmt.Errorf("failed to write record %d: %w", i, err)
		}
	}

	// Write string table
	if _, err := w.Write(stringTable); err != nil {
		return fmt.Errorf("failed to write string table: %w", err)
	}

	return nil
}

// buildStringTable builds the string table and returns it along with a map of string->offset
func buildStringTable(dbc *DBCFile) ([]byte, map[string]uint32) {
	var buf bytes.Buffer
	offsets := make(map[string]uint32)

	// First byte is always null
	buf.WriteByte(0)

	// Collect all unique strings
	for _, record := range dbc.Records {
		for colName, value := range record {
			colDef := dbc.Columns[colName]
			if colDef.Type == dbd.TypeString {
				addStringToTable(&buf, offsets, value)
			} else if colDef.Type == dbd.TypeLocString {
				if locStr, ok := value.(LocString); ok {
					for _, str := range locStr {
						addStringToTable(&buf, offsets, str)
					}
				}
			}
		}
	}

	return buf.Bytes(), offsets
}

// addStringToTable adds a string to the string table if not already present
func addStringToTable(buf *bytes.Buffer, offsets map[string]uint32, value interface{}) {
	var str string
	switch v := value.(type) {
	case string:
		str = v
	case String:
		str = string(v)
	default:
		return
	}

	if str == "" {
		return // Empty strings use offset 0
	}

	if _, exists := offsets[str]; !exists {
		offsets[str] = uint32(buf.Len())
		buf.WriteString(str)
		buf.WriteByte(0) // Null terminator
	}
}

// calculateRecordMetrics calculates the record size and field count
func calculateRecordMetrics(versionDef *dbd.VersionDefinition, columns map[string]dbd.ColumnDefinition) (uint32, uint32) {
	var size uint32
	var fieldCount uint32

	for _, def := range versionDef.Definitions {
		colDef := columns[def.Column]
		arraySize := def.ArraySize
		if arraySize == 0 {
			arraySize = 1
		}

		for i := 0; i < arraySize; i++ {
			switch colDef.Type {
			case dbd.TypeInt, dbd.TypeUInt:
				size += uint32(def.Size / 8)
				fieldCount++
			case dbd.TypeFloat:
				size += 4
				fieldCount++
			case dbd.TypeString:
				size += 4 // String offset
				fieldCount++
			case dbd.TypeLocString:
				size += 4 * 17 // 16 locale strings + 1 flags field
				fieldCount += 17
			}
		}
	}

	return size, fieldCount
}

// writeRecord writes a single record to the writer
func writeRecord(w io.Writer, record Record, versionDef *dbd.VersionDefinition, columns map[string]dbd.ColumnDefinition, stringOffsets map[string]uint32) error {
	for _, def := range versionDef.Definitions {
		colDef, ok := columns[def.Column]
		if !ok {
			return fmt.Errorf("column %s not found in column definitions", def.Column)
		}

		value, ok := record[def.Column]
		if !ok {
			return fmt.Errorf("column %s not found in record", def.Column)
		}

		arraySize := def.ArraySize
		if arraySize == 0 {
			arraySize = 1
		}

		// Write array or single value
		if arraySize > 1 {
			arr, ok := value.([]interface{})
			if !ok {
				return fmt.Errorf("expected array for column %s", def.Column)
			}
			if len(arr) != arraySize {
				return fmt.Errorf("array size mismatch for column %s: expected %d, got %d", def.Column, arraySize, len(arr))
			}
			for i := 0; i < arraySize; i++ {
				if err := writeValue(w, arr[i], colDef.Type, def.Size, def.IsUnsigned, stringOffsets); err != nil {
					return fmt.Errorf("failed to write array element %d of %s: %w", i, def.Column, err)
				}
			}
		} else {
			if err := writeValue(w, value, colDef.Type, def.Size, def.IsUnsigned, stringOffsets); err != nil {
				return fmt.Errorf("failed to write %s: %w", def.Column, err)
			}
		}
	}

	return nil
}

// writeValue writes a single value based on type and size
func writeValue(w io.Writer, value interface{}, colType dbd.ColumnType, size int, isUnsigned bool, stringOffsets map[string]uint32) error {
	switch colType {
	case dbd.TypeInt, dbd.TypeUInt:
		return writeInteger(w, value, size, isUnsigned)
	case dbd.TypeFloat:
		var val float32
		switch v := value.(type) {
		case float32:
			val = v
		case float64:
			val = float32(v)
		case Float32:
			val = float32(v)
		default:
			return fmt.Errorf("invalid float value type: %T", value)
		}
		return binary.Write(w, binary.LittleEndian, val)
	case dbd.TypeString:
		str := ""
		switch v := value.(type) {
		case string:
			str = v
		case String:
			str = string(v)
		default:
			return fmt.Errorf("invalid string value type: %T", value)
		}
		offset := uint32(0)
		if str != "" {
			var ok bool
			offset, ok = stringOffsets[str]
			if !ok {
				return fmt.Errorf("string not found in string table: %s", str)
			}
		}
		return binary.Write(w, binary.LittleEndian, offset)
	case dbd.TypeLocString:
		var locStr LocString
		switch v := value.(type) {
		case LocString:
			locStr = v
		default:
			return fmt.Errorf("invalid locstring value type: %T", value)
		}
		// Write 16 string offsets
		for i := 0; i < 16; i++ {
			offset := uint32(0)
			if locStr[i] != "" {
				var ok bool
				offset, ok = stringOffsets[locStr[i]]
				if !ok {
					return fmt.Errorf("locstring[%d] not found in string table: %s", i, locStr[i])
				}
			}
			if err := binary.Write(w, binary.LittleEndian, offset); err != nil {
				return err
			}
		}
		// Write flags (always 0 for vanilla)
		return binary.Write(w, binary.LittleEndian, uint32(0))
	default:
		return fmt.Errorf("unsupported column type: %s", colType)
	}
}

// writeInteger writes an integer of the specified size and signedness
func writeInteger(w io.Writer, value interface{}, size int, isUnsigned bool) error {
	switch size {
	case 8:
		if isUnsigned {
			var val uint8
			switch v := value.(type) {
			case uint8:
				val = v
			case UInt8:
				val = uint8(v)
			case int, int8, int16, int32, int64, uint, uint16, uint32, uint64:
				val = uint8(toUInt64(v))
			default:
				return fmt.Errorf("invalid uint8 value type: %T", value)
			}
			return binary.Write(w, binary.LittleEndian, val)
		}
		var val int8
		switch v := value.(type) {
		case int8:
			val = v
		case Int8:
			val = int8(v)
		case int, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			val = int8(toInt64(v))
		default:
			return fmt.Errorf("invalid int8 value type: %T", value)
		}
		return binary.Write(w, binary.LittleEndian, val)
	case 16:
		if isUnsigned {
			var val uint16
			switch v := value.(type) {
			case uint16:
				val = v
			case UInt16:
				val = uint16(v)
			case int, int8, int16, int32, int64, uint, uint8, uint32, uint64:
				val = uint16(toUInt64(v))
			default:
				return fmt.Errorf("invalid uint16 value type: %T", value)
			}
			return binary.Write(w, binary.LittleEndian, val)
		}
		var val int16
		switch v := value.(type) {
		case int16:
			val = v
		case Int16:
			val = int16(v)
		case int, int8, int32, int64, uint, uint8, uint16, uint32, uint64:
			val = int16(toInt64(v))
		default:
			return fmt.Errorf("invalid int16 value type: %T", value)
		}
		return binary.Write(w, binary.LittleEndian, val)
	case 32:
		if isUnsigned {
			var val uint32
			switch v := value.(type) {
			case uint32:
				val = v
			case UInt32:
				val = uint32(v)
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint64:
				val = uint32(toUInt64(v))
			default:
				return fmt.Errorf("invalid uint32 value type: %T", value)
			}
			return binary.Write(w, binary.LittleEndian, val)
		}
		var val int32
		switch v := value.(type) {
		case int32:
			val = v
		case Int32:
			val = int32(v)
		case int, int8, int16, int64, uint, uint8, uint16, uint32, uint64:
			val = int32(toInt64(v))
		default:
			return fmt.Errorf("invalid int32 value type: %T", value)
		}
		return binary.Write(w, binary.LittleEndian, val)
	case 64:
		if isUnsigned {
			var val uint64
			switch v := value.(type) {
			case uint64:
				val = v
			case UInt64:
				val = uint64(v)
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32:
				val = toUInt64(v)
			default:
				return fmt.Errorf("invalid uint64 value type: %T", value)
			}
			return binary.Write(w, binary.LittleEndian, val)
		}
		var val int64
		switch v := value.(type) {
		case int64:
			val = v
		case Int64:
			val = int64(v)
		case int, int8, int16, int32, uint, uint8, uint16, uint32, uint64:
			val = toInt64(v)
		default:
			return fmt.Errorf("invalid int64 value type: %T", value)
		}
		return binary.Write(w, binary.LittleEndian, val)
	default:
		return fmt.Errorf("unsupported integer size: %d", size)
	}
}

// Helper functions to convert numeric types to int64/uint64
func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int8:
		return int64(val)
	case int16:
		return int64(val)
	case int32:
		return int64(val)
	case int64:
		return val
	case uint:
		return int64(val)
	case uint8:
		return int64(val)
	case uint16:
		return int64(val)
	case uint32:
		return int64(val)
	case uint64:
		return int64(val)
	default:
		return 0
	}
}

func toUInt64(v interface{}) uint64 {
	switch val := v.(type) {
	case int:
		return uint64(val)
	case int8:
		return uint64(val)
	case int16:
		return uint64(val)
	case int32:
		return uint64(val)
	case int64:
		return uint64(val)
	case uint:
		return uint64(val)
	case uint8:
		return uint64(val)
	case uint16:
		return uint64(val)
	case uint32:
		return uint64(val)
	case uint64:
		return val
	default:
		return 0
	}
}
