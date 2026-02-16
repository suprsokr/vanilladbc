package dbc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/suprsokr/vanilladbc/pkg/dbd"
)

// RecordHandler is called for each record as it's read from the DBC file
type RecordHandler func(recordNum int, record Record) error

// StreamFile reads a DBC file and calls the handler for each record
// This is more memory-efficient than ReadFile for large files
func StreamFile(path string, dbdDef *dbd.DBDefinition, buildVersion string, handler RecordHandler) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	build, err := dbd.NewBuild(buildVersion)
	if err != nil {
		return fmt.Errorf("invalid build version: %w", err)
	}

	return Stream(file, dbdDef, *build, handler)
}

// Stream reads a DBC file and calls the handler for each record
func Stream(r io.Reader, dbdDef *dbd.DBDefinition, build dbd.Build, handler RecordHandler) error {
	// Read entire file into memory (we need random access for string table)
	// TODO: Could optimize this with a seekable reader
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	if len(data) < 20 {
		return fmt.Errorf("file too small to be a valid DBC file")
	}

	// Parse header
	var header Header
	buf := bytes.NewReader(data)

	if err := binary.Read(buf, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	// Verify magic
	if string(header.Magic[:]) != "WDBC" {
		return fmt.Errorf("invalid magic: expected WDBC, got %s", string(header.Magic[:]))
	}

	// Get version definition for this build
	versionDef, err := dbdDef.GetVersionDefinition(build)
	if err != nil {
		return fmt.Errorf("failed to get version definition: %w", err)
	}

	// Calculate string table offset
	stringTableOffset := 20 + (header.RecordCount * header.RecordSize)
	if uint32(len(data)) < stringTableOffset+header.StringTableSize {
		return fmt.Errorf("file too small for declared string table size")
	}

	// Get string table
	stringTable := data[stringTableOffset : stringTableOffset+header.StringTableSize]

	// Read records
	recordData := data[20:stringTableOffset]
	recordBuf := bytes.NewReader(recordData)

	for i := uint32(0); i < header.RecordCount; i++ {
		record, err := readRecord(recordBuf, versionDef, dbdDef.Columns, stringTable)
		if err != nil {
			return fmt.Errorf("failed to read record %d: %w", i, err)
		}
		
		// Call handler for this record
		if err := handler(int(i), record); err != nil {
			return fmt.Errorf("handler failed at record %d: %w", i, err)
		}
	}

	return nil
}

// Iterator provides an iterator interface for reading DBC records
type Iterator struct {
	dbdDef       *dbd.DBDefinition
	versionDef   *dbd.VersionDefinition
	recordBuf    *bytes.Reader
	stringTable  []byte
	recordCount  uint32
	currentIndex uint32
	currentRecord Record
	err          error
}

// NewIterator creates a new iterator for reading DBC records
func NewIterator(r io.Reader, dbdDef *dbd.DBDefinition, build dbd.Build) (*Iterator, error) {
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

	// Get version definition
	versionDef, err := dbdDef.GetVersionDefinition(build)
	if err != nil {
		return nil, fmt.Errorf("failed to get version definition: %w", err)
	}

	// Calculate string table offset
	stringTableOffset := 20 + (header.RecordCount * header.RecordSize)
	if uint32(len(data)) < stringTableOffset+header.StringTableSize {
		return nil, fmt.Errorf("file too small for declared string table size")
	}

	// Get string table and record data
	stringTable := data[stringTableOffset : stringTableOffset+header.StringTableSize]
	recordData := data[20:stringTableOffset]

	return &Iterator{
		dbdDef:       dbdDef,
		versionDef:   versionDef,
		recordBuf:    bytes.NewReader(recordData),
		stringTable:  stringTable,
		recordCount:  header.RecordCount,
		currentIndex: 0,
	}, nil
}

// Next advances the iterator to the next record
// Returns false when there are no more records or an error occurred
func (it *Iterator) Next() bool {
	if it.currentIndex >= it.recordCount {
		return false
	}

	record, err := readRecord(it.recordBuf, it.versionDef, it.dbdDef.Columns, it.stringTable)
	if err != nil {
		it.err = err
		return false
	}

	it.currentRecord = record
	it.currentIndex++
	return true
}

// Record returns the current record
func (it *Iterator) Record() Record {
	return it.currentRecord
}

// Index returns the current record index (0-based)
func (it *Iterator) Index() int {
	return int(it.currentIndex - 1)
}

// Err returns any error that occurred during iteration
func (it *Iterator) Err() error {
	return it.err
}

// Count returns the total number of records
func (it *Iterator) Count() int {
	return int(it.recordCount)
}
