package dbd

import (
	"strings"
	"testing"
)

func TestParseBuild(t *testing.T) {
	tests := []struct {
		input    string
		expected Build
		hasError bool
	}{
		{"1.12.1.5875", Build{1, 12, 1, 5875}, false},
		{"1.0.0.3980", Build{1, 0, 0, 3980}, false},
		{"invalid", Build{}, true},
		{"1.12.1", Build{}, true},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			build, err := NewBuild(test.input)
			if test.hasError {
				if err == nil {
					t.Errorf("expected error for input %s", test.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if build.Major != test.expected.Major || build.Minor != test.expected.Minor ||
				build.Patch != test.expected.Patch || build.Build != test.expected.Build {
				t.Errorf("expected %v, got %v", test.expected, *build)
			}
		})
	}
}

func TestBuildRange(t *testing.T) {
	min := Build{1, 12, 0, 5595}
	max := Build{1, 12, 3, 6141}
	br := BuildRange{Min: min, Max: max}

	tests := []struct {
		build    Build
		expected bool
	}{
		{Build{1, 12, 1, 5875}, true},  // Within range
		{Build{1, 12, 0, 5595}, true},  // Min boundary
		{Build{1, 12, 3, 6141}, true},  // Max boundary
		{Build{1, 11, 0, 5000}, false}, // Before range
		{Build{1, 13, 0, 7000}, false}, // After range
	}

	for _, test := range tests {
		t.Run(test.build.String(), func(t *testing.T) {
			result := br.Contains(test.build)
			if result != test.expected {
				t.Errorf("Contains(%v) = %v, expected %v", test.build, result, test.expected)
			}
		})
	}
}

func TestParseSimpleDBD(t *testing.T) {
	input := `COLUMNS
int ID
string Name
float Speed

BUILD 1.12.1.5875
$id$ID<32>
Name
Speed
`

	dbd, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Check columns
	if len(dbd.Columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(dbd.Columns))
	}

	if col, ok := dbd.Columns["ID"]; !ok {
		t.Error("ID column not found")
	} else if col.Type != TypeInt {
		t.Errorf("ID type: expected int, got %s", col.Type)
	}

	// Check version definitions
	if len(dbd.VersionDefinitions) != 1 {
		t.Fatalf("expected 1 version definition, got %d", len(dbd.VersionDefinitions))
	}

	vd := dbd.VersionDefinitions[0]
	if len(vd.Builds) != 1 {
		t.Errorf("expected 1 build, got %d", len(vd.Builds))
	}

	if len(vd.Definitions) != 3 {
		t.Errorf("expected 3 field definitions, got %d", len(vd.Definitions))
	}

	// Check ID field
	if vd.Definitions[0].Column != "ID" {
		t.Errorf("first field: expected ID, got %s", vd.Definitions[0].Column)
	}
	if !vd.Definitions[0].IsID {
		t.Error("ID field should be marked as ID")
	}
	if vd.Definitions[0].Size != 32 {
		t.Errorf("ID size: expected 32, got %d", vd.Definitions[0].Size)
	}
}

func TestParseBuildRange(t *testing.T) {
	input := `COLUMNS
int ID

BUILD 1.12.0.5595-1.12.3.6141
ID<32>
`

	dbd, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	vd := dbd.VersionDefinitions[0]
	if len(vd.BuildRanges) != 1 {
		t.Fatalf("expected 1 build range, got %d", len(vd.BuildRanges))
	}

	br := vd.BuildRanges[0]
	testBuild := Build{1, 12, 1, 5875}
	if !br.Contains(testBuild) {
		t.Errorf("build range should contain %v", testBuild)
	}
}

func TestParseArray(t *testing.T) {
	input := `COLUMNS
int Reagent

BUILD 1.12.1.5875
Reagent<32>[8]
`

	dbd, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	def := dbd.VersionDefinitions[0].Definitions[0]
	if def.ArraySize != 8 {
		t.Errorf("expected array size 8, got %d", def.ArraySize)
	}
}

func TestGetVersionDefinition(t *testing.T) {
	input := `COLUMNS
int ID

BUILD 1.12.1.5875
ID<32>

BUILD 1.11.0.5000
ID<32>
`

	dbd, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Test matching build
	build := Build{1, 12, 1, 5875}
	vd, err := dbd.GetVersionDefinition(build)
	if err != nil {
		t.Errorf("expected to find version definition for %v", build)
	}
	if !vd.Matches(build) {
		t.Error("version definition should match build")
	}

	// Test non-matching build
	nonMatchBuild := Build{1, 13, 0, 6000}
	_, err = dbd.GetVersionDefinition(nonMatchBuild)
	if err == nil {
		t.Error("expected error for non-matching build")
	}
}
