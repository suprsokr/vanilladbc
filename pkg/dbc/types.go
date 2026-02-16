package dbc

import "github.com/suprsokr/vanilladbc/pkg/dbd"

// Header represents the DBC file header (WDBC format for vanilla)
type Header struct {
	Magic           [4]byte // 'WDBC'
	RecordCount     uint32
	FieldCount      uint32
	RecordSize      uint32
	StringTableSize uint32
}

// Record represents a single record in the DBC file
type Record map[string]interface{}

// DBCFile represents a parsed DBC file
type DBCFile struct {
	Header      Header
	Records     []Record
	StringTable []byte
	Definition  *dbd.VersionDefinition
	Columns     map[string]dbd.ColumnDefinition
}

// Value types that can be stored in a record
type (
	Int8    int8
	Int16   int16
	Int32   int32
	Int64   int64
	UInt8   uint8
	UInt16  uint16
	UInt32  uint32
	UInt64  uint64
	Float32 float32
	String  string
	LocString [16]string // 16 locale strings (enUS, koKR, frFR, deDE, zhCN, zhTW, esES, esMX, ruRU, jaJP, ptPT, itIT, ukUA, ptBR, reserved1, reserved2)
)
