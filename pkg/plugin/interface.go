package plugin

import (
	"github.com/suprsokr/vanilladbc/pkg/dbc"
	"github.com/suprsokr/vanilladbc/pkg/dbd"
)

// Writer is the interface that output plugins must implement
type Writer interface {
	// WriteHeader is called once before any records are written
	// It receives the version definition and column definitions
	WriteHeader(versionDef *dbd.VersionDefinition, columns map[string]dbd.ColumnDefinition) error
	
	// WriteRecord is called for each record in the DBC file
	WriteRecord(record dbc.Record) error
	
	// WriteFooter is called once after all records are written
	// This is where final output should be flushed/finalized
	WriteFooter() error
}

// Reader is the interface that input plugins must implement
type Reader interface {
	// ReadHeader is called once before reading records
	// It should return the version definition and column definitions
	ReadHeader() (*dbd.VersionDefinition, map[string]dbd.ColumnDefinition, error)
	
	// ReadRecord is called repeatedly to read records
	// Returns nil when no more records are available
	ReadRecord() (dbc.Record, error)
	
	// Close is called to cleanup resources
	Close() error
}
