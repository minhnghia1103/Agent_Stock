package sqlite

import (
	"database/sql"
	"fmt"
)

// migrateLegacy adds Phase 10 columns to sessions if upgrading an older DB.
func migrateLegacy(db *sql.DB) error {
	cols, err := tableColumns(db, "sessions")
	if err != nil {
		return err
	}
	need := map[string]string{
		"tenant_id": "TEXT NOT NULL DEFAULT ''",
		"user_id":   "TEXT NOT NULL DEFAULT ''",
		"agent_id":  "TEXT NOT NULL DEFAULT 'default'",
	}
	for name, decl := range need {
		if cols[name] {
			continue
		}
		q := fmt.Sprintf("ALTER TABLE sessions ADD COLUMN %s %s", name, decl)
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("migrate sessions.%s: %w", name, err)
		}
	}
	return nil
}

func tableColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}
