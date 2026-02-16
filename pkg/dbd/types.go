package dbd

import "fmt"

// Build represents a WoW build version (e.g., 1.12.1.5875)
type Build struct {
	Major uint16
	Minor uint16
	Patch uint16
	Build uint32
}

// NewBuild creates a Build from a version string like "1.12.1.5875"
func NewBuild(version string) (*Build, error) {
	var b Build
	_, err := fmt.Sscanf(version, "%d.%d.%d.%d", &b.Major, &b.Minor, &b.Patch, &b.Build)
	if err != nil {
		return nil, fmt.Errorf("invalid build version: %w", err)
	}
	return &b, nil
}

// String returns the build version as a string
func (b Build) String() string {
	return fmt.Sprintf("%d.%d.%d.%d", b.Major, b.Minor, b.Patch, b.Build)
}

// Compare compares two builds. Returns -1 if b < other, 0 if equal, 1 if b > other
func (b Build) Compare(other Build) int {
	if b.Major != other.Major {
		if b.Major < other.Major {
			return -1
		}
		return 1
	}
	if b.Minor != other.Minor {
		if b.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if b.Patch != other.Patch {
		if b.Patch < other.Patch {
			return -1
		}
		return 1
	}
	if b.Build != other.Build {
		if b.Build < other.Build {
			return -1
		}
		return 1
	}
	return 0
}

// BuildRange represents a range of builds (e.g., 1.12.0.5595-1.12.3.6141)
type BuildRange struct {
	Min Build
	Max Build
}

// Contains checks if a build is within the range (inclusive)
func (br BuildRange) Contains(build Build) bool {
	return build.Compare(br.Min) >= 0 && build.Compare(br.Max) <= 0
}

// ColumnType represents the data type of a column
type ColumnType string

const (
	TypeInt       ColumnType = "int"
	TypeUInt      ColumnType = "uint"
	TypeFloat     ColumnType = "float"
	TypeString    ColumnType = "string"
	TypeLocString ColumnType = "locstring"
)

// ColumnDefinition defines a column in the COLUMNS section
type ColumnDefinition struct {
	Type          ColumnType
	Name          string
	ForeignTable  string // Foreign key table (if any)
	ForeignColumn string // Foreign key column (if any)
	Verified      bool   // false if name ends with ?
	Comment       string
}

// Definition represents a single field definition in a version block
type Definition struct {
	Column     string   // Column name
	Size       int      // Size in bits (8, 16, 32, 64) for int/uint
	IsUnsigned bool     // true if unsigned
	ArraySize  int      // Array length (0 if not an array)
	IsID       bool     // true if marked with $id$
	IsRelation bool     // true if marked with $relation$
	IsNonInline bool    // true if marked with $noninline$
	Annotations []string // All annotations
	Comment    string
}

// VersionDefinition represents a version-specific definition block
type VersionDefinition struct {
	Builds      []Build      // Single builds
	BuildRanges []BuildRange // Build ranges
	LayoutHashes []string     // Layout hashes
	Comment     string
	Definitions []Definition // Field definitions for this version
}

// Matches checks if this version definition applies to a given build
func (vd VersionDefinition) Matches(build Build) bool {
	// Check single builds
	for _, b := range vd.Builds {
		if b.Compare(build) == 0 {
			return true
		}
	}
	
	// Check build ranges
	for _, br := range vd.BuildRanges {
		if br.Contains(build) {
			return true
		}
	}
	
	return false
}

// DBDefinition represents a complete parsed DBD file
type DBDefinition struct {
	Columns             map[string]ColumnDefinition
	VersionDefinitions  []VersionDefinition
}

// GetVersionDefinition finds the version definition matching the given build
func (dbd DBDefinition) GetVersionDefinition(build Build) (*VersionDefinition, error) {
	for i := range dbd.VersionDefinitions {
		if dbd.VersionDefinitions[i].Matches(build) {
			return &dbd.VersionDefinitions[i], nil
		}
	}
	return nil, fmt.Errorf("no version definition found for build %s", build.String())
}
