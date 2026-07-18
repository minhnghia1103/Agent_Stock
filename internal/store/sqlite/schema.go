package sqlite

import _ "embed"

//go:embed schema.sql
var schemaSQL string

// SchemaSQL is the embedded Phase 2 DDL.
func SchemaSQL() string {
	return schemaSQL
}
