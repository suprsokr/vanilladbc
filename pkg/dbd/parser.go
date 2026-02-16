package dbd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ParseFile parses a DBD file from the given path
func ParseFile(path string) (*DBDefinition, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	return Parse(file)
}

// Parse parses a DBD file from an io.Reader
func Parse(r io.Reader) (*DBDefinition, error) {
	scanner := bufio.NewScanner(r)
	
	dbd := &DBDefinition{
		Columns: make(map[string]ColumnDefinition),
		VersionDefinitions: []VersionDefinition{},
	}
	
	var lineNum int
	
	// Read first line - should be "COLUMNS"
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty file")
	}
	lineNum++
	
	line := strings.TrimSpace(scanner.Text())
	if line != "COLUMNS" {
		return nil, fmt.Errorf("line %d: expected COLUMNS, got: %s", lineNum, line)
	}
	
	// Parse column definitions
	for scanner.Scan() {
		lineNum++
		line = scanner.Text()
		
		// Empty line marks end of column definitions
		if strings.TrimSpace(line) == "" {
			break
		}
		
		colDef, err := parseColumnDefinition(line, lineNum)
		if err != nil {
			return nil, err
		}
		
		dbd.Columns[colDef.Name] = colDef
	}
	
	// Parse version definitions
	var currentVersion *VersionDefinition
	var currentDefinitions []Definition
	
	for scanner.Scan() {
		lineNum++
		line = scanner.Text()
		trimmed := strings.TrimSpace(line)
		
		// Empty line marks end of version block
		if trimmed == "" {
			if currentVersion != nil {
				currentVersion.Definitions = currentDefinitions
				dbd.VersionDefinitions = append(dbd.VersionDefinitions, *currentVersion)
				currentVersion = nil
				currentDefinitions = nil
			}
			continue
		}
		
		// Parse BUILD line
		if strings.HasPrefix(trimmed, "BUILD ") {
			if currentVersion == nil {
				currentVersion = &VersionDefinition{}
			}
			builds, buildRanges, err := parseBuildLine(trimmed[6:], lineNum)
			if err != nil {
				return nil, err
			}
			currentVersion.Builds = append(currentVersion.Builds, builds...)
			currentVersion.BuildRanges = append(currentVersion.BuildRanges, buildRanges...)
			continue
		}
		
		// Parse LAYOUT line
		if strings.HasPrefix(trimmed, "LAYOUT ") {
			if currentVersion == nil {
				currentVersion = &VersionDefinition{}
			}
			layouts := parseLayoutLine(trimmed[7:])
			currentVersion.LayoutHashes = append(currentVersion.LayoutHashes, layouts...)
			continue
		}
		
		// Parse COMMENT line
		if strings.HasPrefix(trimmed, "COMMENT ") {
			if currentVersion == nil {
				currentVersion = &VersionDefinition{}
			}
			currentVersion.Comment = strings.TrimSpace(trimmed[8:])
			continue
		}
		
		// Parse field definition
		if currentVersion != nil {
			def, err := parseDefinition(trimmed, lineNum)
			if err != nil {
				return nil, err
			}
			currentDefinitions = append(currentDefinitions, def)
		}
	}
	
	// Add last version definition if exists
	if currentVersion != nil {
		currentVersion.Definitions = currentDefinitions
		dbd.VersionDefinitions = append(dbd.VersionDefinitions, *currentVersion)
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}
	
	return dbd, nil
}

// parseColumnDefinition parses a line like "int ID" or "int<SpellRuneCost::ID> RuneCostID"
func parseColumnDefinition(line string, lineNum int) (ColumnDefinition, error) {
	def := ColumnDefinition{Verified: true}
	
	// Remove comment if present
	if idx := strings.Index(line, "//"); idx >= 0 {
		def.Comment = strings.TrimSpace(line[idx+2:])
		line = line[:idx]
	}
	
	line = strings.TrimSpace(line)
	
	// Parse type
	parts := strings.FieldsFunc(line, func(r rune) bool {
		return r == ' ' || r == '<'
	})
	
	if len(parts) == 0 {
		return def, fmt.Errorf("line %d: empty column definition", lineNum)
	}
	
	def.Type = ColumnType(parts[0])
	
	// Check for foreign key
	if strings.Contains(line, "<") && strings.Contains(line, "::") {
		start := strings.Index(line, "<")
		end := strings.Index(line, ">")
		if end > start {
			fk := line[start+1:end]
			fkParts := strings.Split(fk, "::")
			if len(fkParts) == 2 {
				def.ForeignTable = fkParts[0]
				def.ForeignColumn = fkParts[1]
			}
		}
		// Remove foreign key from line for name parsing
		line = line[:start] + line[end+1:]
	}
	
	// Parse name (last word)
	words := strings.Fields(line)
	if len(words) < 2 {
		return def, fmt.Errorf("line %d: invalid column definition", lineNum)
	}
	
	name := words[len(words)-1]
	
	// Check if unverified (ends with ?)
	if strings.HasSuffix(name, "?") {
		def.Verified = false
		name = name[:len(name)-1]
	}
	
	def.Name = name
	
	return def, nil
}

// parseBuildLine parses a BUILD line like "1.12.0.5595-1.12.3.6141, 1.13.0.5000"
func parseBuildLine(buildStr string, lineNum int) ([]Build, []BuildRange, error) {
	var builds []Build
	var buildRanges []BuildRange
	
	// Split by comma
	parts := strings.Split(buildStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		
		// Check if it's a range
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, nil, fmt.Errorf("line %d: invalid build range: %s", lineNum, part)
			}
			
			minBuild, err := NewBuild(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, nil, fmt.Errorf("line %d: %w", lineNum, err)
			}
			
			maxBuild, err := NewBuild(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, nil, fmt.Errorf("line %d: %w", lineNum, err)
			}
			
			buildRanges = append(buildRanges, BuildRange{Min: *minBuild, Max: *maxBuild})
		} else {
			// Single build
			build, err := NewBuild(part)
			if err != nil {
				return nil, nil, fmt.Errorf("line %d: %w", lineNum, err)
			}
			builds = append(builds, *build)
		}
	}
	
	return builds, buildRanges, nil
}

// parseLayoutLine parses a LAYOUT line like "ABC123, DEF456"
func parseLayoutLine(layoutStr string) []string {
	var layouts []string
	parts := strings.Split(layoutStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			layouts = append(layouts, part)
		}
	}
	return layouts
}

// parseDefinition parses a field definition like "$id$ID<32>" or "Name" or "Reagent<32>[8]"
func parseDefinition(line string, lineNum int) (Definition, error) {
	def := Definition{}
	
	// Remove comment if present
	if idx := strings.Index(line, "//"); idx >= 0 {
		def.Comment = strings.TrimSpace(line[idx+2:])
		line = line[:idx]
	}
	
	line = strings.TrimSpace(line)
	
	// Parse annotations (between $ and $)
	if strings.HasPrefix(line, "$") {
		end := strings.Index(line[1:], "$")
		if end >= 0 {
			annotationStr := line[1:end+1]
			annotations := strings.Split(annotationStr, ",")
			for _, ann := range annotations {
				ann = strings.TrimSpace(ann)
				def.Annotations = append(def.Annotations, ann)
				
				switch ann {
				case "id":
					def.IsID = true
				case "relation":
					def.IsRelation = true
				case "noninline":
					def.IsNonInline = true
				}
			}
			line = line[end+2:]
		}
	}
	
	// Parse size (between < and >)
	if strings.Contains(line, "<") {
		start := strings.Index(line, "<")
		end := strings.Index(line, ">")
		if end > start {
			sizeStr := line[start+1:end]
			
			// Check if unsigned
			if strings.HasPrefix(sizeStr, "u") {
				def.IsUnsigned = true
				sizeStr = sizeStr[1:]
			}
			
			size, err := strconv.Atoi(sizeStr)
			if err != nil {
				return def, fmt.Errorf("line %d: invalid size: %s", lineNum, sizeStr)
			}
			def.Size = size
			
			// Remove size from line
			line = line[:start] + line[end+1:]
		}
	}
	
	// Parse array size (between [ and ])
	if strings.Contains(line, "[") {
		start := strings.Index(line, "[")
		end := strings.Index(line, "]")
		if end > start {
			arrSizeStr := line[start+1:end]
			arrSize, err := strconv.Atoi(arrSizeStr)
			if err != nil {
				return def, fmt.Errorf("line %d: invalid array size: %s", lineNum, arrSizeStr)
			}
			def.ArraySize = arrSize
			
			// Remove array size from line
			line = line[:start] + line[end+1:]
		}
	}
	
	// What's left is the column name
	def.Column = strings.TrimSpace(line)
	
	return def, nil
}
