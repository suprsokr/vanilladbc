package dbc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/suprsokr/vanilladbc/pkg/dbd"
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
	localeCount := dbc.LocaleCount
	if localeCount == 0 {
		localeCount = 16
	}

	stringTable, stringOffsets := buildStringTable(dbc, localeCount)

	dbc.Header.Magic = [4]byte{'W', 'D', 'B', 'C'}
	dbc.Header.RecordCount = uint32(len(dbc.Records))
	dbc.Header.StringTableSize = uint32(len(stringTable))

	recordSize, fieldCount := calculateRecordMetrics(dbc.Definition, dbc.Columns, localeCount)
	dbc.Header.RecordSize = recordSize
	dbc.Header.FieldCount = fieldCount

	if err := binary.Write(w, binary.LittleEndian, &dbc.Header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	for i, record := range dbc.Records {
		if err := writeRecord(w, record, dbc.Definition, dbc.Columns, stringOffsets, localeCount); err != nil {
			return fmt.Errorf("failed to write record %d: %w", i, err)
		}
	}

	if _, err := w.Write(stringTable); err != nil {
		return fmt.Errorf("failed to write string table: %w", err)
	}

	return nil
}

func buildStringTable(dbc *DBCFile, localeCount int) ([]byte, map[string]uint32) {
	var buf bytes.Buffer
	offsets := make(map[string]uint32)

	buf.WriteByte(0)

	for _, record := range dbc.Records {
		for colName, value := range record {
			colDef := dbc.Columns[colName]
			if colDef.Type == dbd.TypeString {
				addStringToTable(&buf, offsets, value)
			} else if colDef.Type == dbd.TypeLocString {
				if locStr, ok := value.(LocString); ok {
					for i := 0; i < len(locStr) && i < localeCount; i++ {
						addStringToTable(&buf, offsets, locStr[i])
					}
				}
			}
		}
	}

	return buf.Bytes(), offsets
}

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
		return
	}

	if _, exists := offsets[str]; !exists {
		offsets[str] = uint32(buf.Len())
		buf.WriteString(str)
		buf.WriteByte(0)
	}
}

func calculateRecordMetrics(versionDef *dbd.VersionDefinition, columns map[string]dbd.ColumnDefinition, localeCount int) (uint32, uint32) {
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
				size += 4
				fieldCount++
			case dbd.TypeLocString:
				size += uint32(4 * (localeCount + 1)) // N locales + 1 flags
				fieldCount += uint32(localeCount + 1)
			}
		}
	}

	return size, fieldCount
}

func writeRecord(w io.Writer, record Record, versionDef *dbd.VersionDefinition, columns map[string]dbd.ColumnDefinition, stringOffsets map[string]uint32, localeCount int) error {
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

		if arraySize > 1 {
			arr, ok := value.([]interface{})
			if !ok {
				return fmt.Errorf("expected array for column %s", def.Column)
			}
			if len(arr) != arraySize {
				return fmt.Errorf("array size mismatch for column %s: expected %d, got %d", def.Column, arraySize, len(arr))
			}
			for i := 0; i < arraySize; i++ {
				if err := writeValue(w, arr[i], colDef.Type, def.Size, def.IsUnsigned, stringOffsets, localeCount); err != nil {
					return fmt.Errorf("failed to write array element %d of %s: %w", i, def.Column, err)
				}
			}
		} else {
			if err := writeValue(w, value, colDef.Type, def.Size, def.IsUnsigned, stringOffsets, localeCount); err != nil {
				return fmt.Errorf("failed to write %s: %w", def.Column, err)
			}
		}
	}

	return nil
}

func writeValue(w io.Writer, value interface{}, colType dbd.ColumnType, size int, isUnsigned bool, stringOffsets map[string]uint32, localeCount int) error {
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
		for i := 0; i < localeCount; i++ {
			offset := uint32(0)
			if i < len(locStr) && locStr[i] != "" {
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
		// Flags
		return binary.Write(w, binary.LittleEndian, uint32(0))
	default:
		return fmt.Errorf("unsupported column type: %s", colType)
	}
}

func writeInteger(w io.Writer, value interface{}, size int, isUnsigned bool) error {
	switch size {
	case 8:
		if isUnsigned {
			return binary.Write(w, binary.LittleEndian, uint8(toUInt64(value)))
		}
		return binary.Write(w, binary.LittleEndian, int8(toInt64(value)))
	case 16:
		if isUnsigned {
			return binary.Write(w, binary.LittleEndian, uint16(toUInt64(value)))
		}
		return binary.Write(w, binary.LittleEndian, int16(toInt64(value)))
	case 32:
		if isUnsigned {
			return binary.Write(w, binary.LittleEndian, uint32(toUInt64(value)))
		}
		return binary.Write(w, binary.LittleEndian, int32(toInt64(value)))
	case 64:
		if isUnsigned {
			return binary.Write(w, binary.LittleEndian, uint64(toUInt64(value)))
		}
		return binary.Write(w, binary.LittleEndian, int64(toInt64(value)))
	default:
		return fmt.Errorf("unsupported integer size: %d", size)
	}
}

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
	// Library-defined typed aliases
	case Int8:
		return int64(val)
	case Int16:
		return int64(val)
	case Int32:
		return int64(val)
	case Int64:
		return int64(val)
	case UInt8:
		return int64(val)
	case UInt16:
		return int64(val)
	case UInt32:
		return int64(val)
	case UInt64:
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
	// Library-defined typed aliases
	case Int8:
		return uint64(val)
	case Int16:
		return uint64(val)
	case Int32:
		return uint64(val)
	case Int64:
		return uint64(val)
	case UInt8:
		return uint64(val)
	case UInt16:
		return uint64(val)
	case UInt32:
		return uint64(val)
	case UInt64:
		return uint64(val)
	default:
		return 0
	}
}
