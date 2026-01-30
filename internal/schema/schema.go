package schema

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// TableSchema represents a database table schema
type TableSchema struct {
	Name    string
	Columns map[string]string // column name -> SQL type
}

// Schema represents the complete schema from a Lua script
type Schema struct {
	Tables map[string]*TableSchema
}

// validIdentifier ensures table/column names are safe for SQL
var validIdentifier = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// LoadFromLuaScript loads schema definitions from a Lua script file
func LoadFromLuaScript(scriptPath string) (*Schema, error) {
	L := lua.NewState()
	defer L.Close()

	// Load the Lua script
	if err := L.DoFile(scriptPath); err != nil {
		return nil, fmt.Errorf("failed to load Lua script: %w", err)
	}

	// Check for schema global variable
	schemaLV := L.GetGlobal("schema")
	if schemaLV.Type() == lua.LTNil {
		// No schema defined - this is OK for passthrough routes
		return &Schema{Tables: make(map[string]*TableSchema)}, nil
	}

	if schemaLV.Type() != lua.LTTable {
		return nil, fmt.Errorf("schema must be a table")
	}

	schemaTable := schemaLV.(*lua.LTable)

	// Get the tables field
	tablesLV := schemaTable.RawGetString("tables")
	if tablesLV.Type() == lua.LTNil {
		return &Schema{Tables: make(map[string]*TableSchema)}, nil
	}

	if tablesLV.Type() != lua.LTTable {
		return nil, fmt.Errorf("schema.tables must be a table")
	}

	schema := &Schema{
		Tables: make(map[string]*TableSchema),
	}

	tablesTable := tablesLV.(*lua.LTable)
	tablesTable.ForEach(func(key, value lua.LValue) {
		tableName, ok := key.(lua.LString)
		if !ok {
			return
		}

		if value.Type() != lua.LTTable {
			return
		}

		// Validate table name
		tableNameStr := string(tableName)
		if !validIdentifier.MatchString(tableNameStr) {
			return
		}

		tableSchema := &TableSchema{
			Name:    tableNameStr,
			Columns: make(map[string]string),
		}

		columnsTable := value.(*lua.LTable)
		columnsTable.ForEach(func(colKey, colValue lua.LValue) {
			colName, ok := colKey.(lua.LString)
			if !ok {
				return
			}

			colType, ok := colValue.(lua.LString)
			if !ok {
				return
			}

			// Validate column name
			colNameStr := string(colName)
			if !validIdentifier.MatchString(colNameStr) {
				return
			}

			tableSchema.Columns[colNameStr] = string(colType)
		})

		schema.Tables[tableNameStr] = tableSchema
	})

	return schema, nil
}

// GenerateSQL generates CREATE TABLE statements for the schema
func (s *Schema) GenerateSQL() string {
	if len(s.Tables) == 0 {
		return ""
	}

	var sb strings.Builder

	// Sort table names for deterministic output
	tableNames := make([]string, 0, len(s.Tables))
	for name := range s.Tables {
		tableNames = append(tableNames, name)
	}
	sort.Strings(tableNames)

	for _, tableName := range tableNames {
		table := s.Tables[tableName]
		sb.WriteString(table.GenerateCreateTable())
		sb.WriteString("\n\n")
	}

	return strings.TrimSpace(sb.String())
}

// GenerateCreateTable generates a CREATE TABLE statement for this table
func (t *TableSchema) GenerateCreateTable() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n", t.Name))

	// Sort column names for deterministic output
	colNames := make([]string, 0, len(t.Columns))
	for name := range t.Columns {
		colNames = append(colNames, name)
	}
	sort.Strings(colNames)

	for i, colName := range colNames {
		colType := t.Columns[colName]
		sb.WriteString(fmt.Sprintf("  %s %s", colName, colType))
		if i < len(colNames)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(");")

	return sb.String()
}

// Merge combines multiple schemas into one
func Merge(schemas ...*Schema) *Schema {
	merged := &Schema{
		Tables: make(map[string]*TableSchema),
	}

	for _, s := range schemas {
		if s == nil {
			continue
		}
		for tableName, tableSchema := range s.Tables {
			// If table already exists, merge columns
			if existing, ok := merged.Tables[tableName]; ok {
				for colName, colType := range tableSchema.Columns {
					// Don't overwrite existing columns
					if _, exists := existing.Columns[colName]; !exists {
						existing.Columns[colName] = colType
					}
				}
			} else {
				// Deep copy the table schema
				newTable := &TableSchema{
					Name:    tableSchema.Name,
					Columns: make(map[string]string),
				}
				for colName, colType := range tableSchema.Columns {
					newTable.Columns[colName] = colType
				}
				merged.Tables[tableName] = newTable
			}
		}
	}

	return merged
}

// ValidateRecord checks if a record matches the schema
func (t *TableSchema) ValidateRecord(columns map[string]interface{}) error {
	for colName := range columns {
		if _, ok := t.Columns[colName]; !ok {
			return fmt.Errorf("column '%s' not declared in schema for table '%s'", colName, t.Name)
		}
	}
	return nil
}
