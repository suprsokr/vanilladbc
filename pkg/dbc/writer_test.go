package dbc

import (
	"bytes"
	"strings"
	"testing"

	"github.com/suprsokr/vanilladbc/pkg/dbd"
)

// minimalDBD builds a simple in-memory DBDefinition for testing.
func minimalDBD(t *testing.T, dbdSrc string) *dbd.DBDefinition {
	t.Helper()
	def, err := dbd.Parse(strings.NewReader(dbdSrc))
	if err != nil {
		t.Fatalf("failed to parse test DBD: %v", err)
	}
	return def
}

// roundtrip writes dbcFile to a buffer and reads it back, returning the result.
func roundtrip(t *testing.T, dbcFile *DBCFile) *DBCFile {
	t.Helper()
	var buf bytes.Buffer
	if err := Write(&buf, dbcFile); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	build, err := dbd.NewBuild("1.12.1.5875")
	if err != nil {
		t.Fatalf("invalid build: %v", err)
	}
	result, err := Read(bytes.NewReader(buf.Bytes()), &dbd.DBDefinition{
		Columns:            dbcFile.Columns,
		VersionDefinitions: []dbd.VersionDefinition{*dbcFile.Definition},
	}, *build)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	return result
}

const simpleDBD = `COLUMNS
int ID
int Value
uint UValue
float Speed
string Name
locstring Label

BUILD 1.12.1.5875
$id$ID<32>
Value<32>
UValue<32>
Speed
Name
Label
`

// makeSimpleDBCFile creates a DBCFile with one record using primitive Go types
// (the types returned by the reader).
func makeSimpleDBCFile(t *testing.T, id, value int32, uvalue uint32, speed float32, name, label string) *DBCFile {
	t.Helper()
	def := minimalDBD(t, simpleDBD)
	build, _ := dbd.NewBuild("1.12.1.5875")
	vd, err := def.GetVersionDefinition(*build)
	if err != nil {
		t.Fatalf("GetVersionDefinition: %v", err)
	}
	locStr := make(LocString, 8)
	locStr[0] = label
	return &DBCFile{
		Definition:  vd,
		Columns:     def.Columns,
		LocaleCount: 8,
		Records: []Record{
			{
				"ID":     id,
				"Value":  value,
				"UValue": uvalue,
				"Speed":  speed,
				"Name":   name,
				"Label":  locStr,
			},
		},
	}
}

// TestRoundtrip_PrimitiveTypes verifies a basic write→read roundtrip using
// plain Go primitive types (int32, uint32, float32, string) — the types the
// reader itself produces.
func TestRoundtrip_PrimitiveTypes(t *testing.T) {
	f := makeSimpleDBCFile(t, 42, -7, 99, 3.14, "hello", "world")
	got := roundtrip(t, f)

	if len(got.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(got.Records))
	}
	rec := got.Records[0]

	checkInt32(t, "ID", rec["ID"], 42)
	checkInt32(t, "Value", rec["Value"], -7)
	checkUint32(t, "UValue", rec["UValue"], 99)
	checkFloat32(t, "Speed", rec["Speed"], 3.14)
	checkString(t, "Name", rec["Name"], "hello")
	checkLocString(t, "Label", rec["Label"], "world")
}

// TestRoundtrip_LibraryTypedAliases verifies that record fields set using the
// library's own typed aliases (Int32, UInt32, etc.) are correctly serialised.
// This is the regression test for the bug where toInt64/toUInt64 didn't handle
// the library's custom types and silently wrote 0.
func TestRoundtrip_LibraryTypedAliases(t *testing.T) {
	def := minimalDBD(t, simpleDBD)
	build, _ := dbd.NewBuild("1.12.1.5875")
	vd, err := def.GetVersionDefinition(*build)
	if err != nil {
		t.Fatalf("GetVersionDefinition: %v", err)
	}

	locStr := make(LocString, 8)
	locStr[0] = "alias_label"

	f := &DBCFile{
		Definition:  vd,
		Columns:     def.Columns,
		LocaleCount: 8,
		Records: []Record{
			{
				"ID":     Int32(100),   // library alias — was broken before fix
				"Value":  Int32(-50),   // library alias
				"UValue": UInt32(200),  // library alias
				"Speed":  Float32(1.5), // library alias
				"Name":   String("lib_name"),
				"Label":  locStr,
			},
		},
	}

	got := roundtrip(t, f)

	if len(got.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(got.Records))
	}
	rec := got.Records[0]

	checkInt32(t, "ID", rec["ID"], 100)
	checkInt32(t, "Value", rec["Value"], -50)
	checkUint32(t, "UValue", rec["UValue"], 200)
	checkFloat32(t, "Speed", rec["Speed"], 1.5)
	checkString(t, "Name", rec["Name"], "lib_name")
	checkLocString(t, "Label", rec["Label"], "alias_label")
}

// TestRoundtrip_AllLibraryIntTypes checks every library integer alias size.
// Uses 32-bit fields for all types since the DBC format commonly uses 32-bit
// storage even for smaller logical values. This exercises that all library alias
// types are correctly handled by toInt64/toUInt64 in the writer.
const intTypesDBD = `COLUMNS
int ID
int I8
int I16
int I32
int I64
uint U8
uint U16
uint U32
uint U64

BUILD 1.12.1.5875
$id$ID<32>
I8<32>
I16<32>
I32<32>
I64<32>
U8<32>
U16<32>
U32<32>
U64<32>
`

func TestRoundtrip_AllLibraryIntTypes(t *testing.T) {
	def := minimalDBD(t, intTypesDBD)
	build, _ := dbd.NewBuild("1.12.1.5875")
	vd, _ := def.GetVersionDefinition(*build)

	f := &DBCFile{
		Definition:  vd,
		Columns:     def.Columns,
		LocaleCount: 8,
		Records: []Record{
			{
				"ID":  Int32(1),
				"I8":  Int8(127),   // library alias written as 32-bit signed
				"I16": Int16(-1000), // library alias
				"I32": Int32(-1000000),
				"I64": Int64(9999999), // stored as 32-bit, value fits in int32
				"U8":  UInt8(200),
				"U16": UInt16(60000),
				"U32": UInt32(60001), // stored as 32-bit unsigned, use value that fits
				"U64": UInt64(60002), // stored as 32-bit unsigned, use value that fits
			},
		},
	}

	got := roundtrip(t, f)

	if len(got.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(got.Records))
	}
	rec := got.Records[0]

	checkInt32(t, "ID", rec["ID"], 1)

	// All written via library alias types — before the fix these would all be 0
	if v := toInt64(rec["I8"]); v != 127 {
		t.Errorf("I8: expected 127, got %d", v)
	}
	if v := toInt64(rec["I16"]); v != -1000 {
		t.Errorf("I16: expected -1000, got %d", v)
	}
	if v := toInt64(rec["I32"]); v != -1000000 {
		t.Errorf("I32: expected -1000000, got %d", v)
	}
	if v := toInt64(rec["I64"]); v != 9999999 {
		t.Errorf("I64: expected 9999999, got %d", v)
	}
	if v := toUInt64(rec["U8"]); v != 200 {
		t.Errorf("U8: expected 200, got %d", v)
	}
	if v := toUInt64(rec["U16"]); v != 60000 {
		t.Errorf("U16: expected 60000, got %d", v)
	}
	if v := toUInt64(rec["U32"]); v != 60001 {
		t.Errorf("U32: expected 60001, got %d", v)
	}
	if v := toUInt64(rec["U64"]); v != 60002 {
		t.Errorf("U64: expected 60002, got %d", v)
	}
}

// TestRoundtrip_MultipleRecords ensures multiple records are written and read back correctly.
func TestRoundtrip_MultipleRecords(t *testing.T) {
	def := minimalDBD(t, simpleDBD)
	build, _ := dbd.NewBuild("1.12.1.5875")
	vd, _ := def.GetVersionDefinition(*build)

	records := []struct {
		id    int32
		value int32
		name  string
	}{
		{1, 10, "alpha"},
		{2, 20, "beta"},
		{3, 30, "gamma"},
	}

	var recs []Record
	for _, r := range records {
		locStr := make(LocString, 8)
		recs = append(recs, Record{
			"ID":     r.id,
			"Value":  r.value,
			"UValue": uint32(0),
			"Speed":  float32(0),
			"Name":   r.name,
			"Label":  locStr,
		})
	}

	f := &DBCFile{
		Definition:  vd,
		Columns:     def.Columns,
		LocaleCount: 8,
		Records:     recs,
	}

	got := roundtrip(t, f)

	if len(got.Records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(got.Records))
	}
	for i, r := range records {
		checkInt32(t, "ID", got.Records[i]["ID"], r.id)
		checkInt32(t, "Value", got.Records[i]["Value"], r.value)
		checkString(t, "Name", got.Records[i]["Name"], r.name)
	}
}

// TestToInt64_LibraryTypes unit-tests the toInt64 helper directly for all custom types.
func TestToInt64_LibraryTypes(t *testing.T) {
	cases := []struct {
		name     string
		input    interface{}
		expected int64
	}{
		{"Int8", Int8(127), 127},
		{"Int8 negative", Int8(-128), -128},
		{"Int16", Int16(32767), 32767},
		{"Int16 negative", Int16(-32768), -32768},
		{"Int32", Int32(2147483647), 2147483647},
		{"Int32 negative", Int32(-2147483648), -2147483648},
		{"Int64", Int64(9223372036854775807), 9223372036854775807},
		{"UInt8", UInt8(255), 255},
		{"UInt16", UInt16(65535), 65535},
		{"UInt32", UInt32(4294967295), 4294967295},
		{"UInt64", UInt64(1000), 1000},
		// Plain Go types still work
		{"int32", int32(42), 42},
		{"uint32", uint32(99), 99},
		// Unknown type returns 0
		{"unknown", "not a number", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := toInt64(tc.input)
			if got != tc.expected {
				t.Errorf("toInt64(%T(%v)) = %d, want %d", tc.input, tc.input, got, tc.expected)
			}
		})
	}
}

// TestToUInt64_LibraryTypes unit-tests the toUInt64 helper directly for all custom types.
func TestToUInt64_LibraryTypes(t *testing.T) {
	cases := []struct {
		name     string
		input    interface{}
		expected uint64
	}{
		{"UInt8", UInt8(255), 255},
		{"UInt16", UInt16(65535), 65535},
		{"UInt32", UInt32(4294967295), 4294967295},
		{"UInt64", UInt64(18446744073709551615), 18446744073709551615},
		{"Int8", Int8(100), 100},
		{"Int16", Int16(1000), 1000},
		{"Int32", Int32(100000), 100000},
		{"Int64", Int64(999999), 999999},
		// Plain Go types still work
		{"int32", int32(7), 7},
		{"uint32", uint32(42), 42},
		// Unknown type returns 0
		{"unknown", struct{}{}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := toUInt64(tc.input)
			if got != tc.expected {
				t.Errorf("toUInt64(%T(%v)) = %d, want %d", tc.input, tc.input, got, tc.expected)
			}
		})
	}
}

// --- helpers ---

func checkInt32(t *testing.T, field string, got interface{}, want int32) {
	t.Helper()
	v := int32(toInt64(got))
	if v != want {
		t.Errorf("%s: expected %d, got %d (type %T)", field, want, v, got)
	}
}

func checkUint32(t *testing.T, field string, got interface{}, want uint32) {
	t.Helper()
	v := uint32(toUInt64(got))
	if v != want {
		t.Errorf("%s: expected %d, got %d (type %T)", field, want, v, got)
	}
}

func checkFloat32(t *testing.T, field string, got interface{}, want float32) {
	t.Helper()
	var v float32
	switch val := got.(type) {
	case float32:
		v = val
	case Float32:
		v = float32(val)
	default:
		t.Errorf("%s: unexpected type %T", field, got)
		return
	}
	if v != want {
		t.Errorf("%s: expected %f, got %f", field, want, v)
	}
}

func checkString(t *testing.T, field string, got interface{}, want string) {
	t.Helper()
	switch v := got.(type) {
	case string:
		if v != want {
			t.Errorf("%s: expected %q, got %q", field, want, v)
		}
	case String:
		if string(v) != want {
			t.Errorf("%s: expected %q, got %q", field, want, string(v))
		}
	default:
		t.Errorf("%s: unexpected type %T", field, got)
	}
}

func checkLocString(t *testing.T, field string, got interface{}, want string) {
	t.Helper()
	ls, ok := got.(LocString)
	if !ok {
		t.Errorf("%s: expected LocString, got %T", field, got)
		return
	}
	if len(ls) == 0 || ls[0] != want {
		t.Errorf("%s: expected enUS=%q, got %v", field, want, ls)
	}
}
