package index

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const sqliteCacheSchema = `
CREATE TABLE IF NOT EXISTS meta (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS files (
	path TEXT PRIMARY KEY,
	language TEXT NOT NULL,
	size_bytes INTEGER NOT NULL,
	mod_time_unix_nano INTEGER NOT NULL,
	estimated_tokens INTEGER NOT NULL,
	package_name TEXT NOT NULL,
	imports_json TEXT NOT NULL,
	symbols_json TEXT NOT NULL,
	is_test INTEGER NOT NULL,
	is_route INTEGER NOT NULL,
	is_config INTEGER NOT NULL
);
`

func saveSQLiteCache(idx *RepoIndex, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(sqliteCacheSchema); err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM meta`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM files`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO meta(key, value) VALUES('schema_version', '1')`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
INSERT INTO files(
	path,
	language,
	size_bytes,
	mod_time_unix_nano,
	estimated_tokens,
	package_name,
	imports_json,
	symbols_json,
	is_test,
	is_route,
	is_config
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, file := range idx.Files {
		importsJSON, err := json.Marshal(file.Imports)
		if err != nil {
			return err
		}
		symbolsJSON, err := json.Marshal(file.Symbols)
		if err != nil {
			return err
		}
		if _, err := stmt.Exec(
			file.Path,
			file.Language,
			file.SizeBytes,
			file.ModTimeUnixNano,
			file.EstimatedTokens,
			file.Package,
			string(importsJSON),
			string(symbolsJSON),
			boolInt(file.IsTest),
			boolInt(file.IsRoute),
			boolInt(file.IsConfig),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func loadSQLiteCache(path string) (map[string]FileInfo, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
SELECT
	path,
	language,
	size_bytes,
	mod_time_unix_nano,
	estimated_tokens,
	package_name,
	imports_json,
	symbols_json,
	is_test,
	is_route,
	is_config
FROM files
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]FileInfo{}
	for rows.Next() {
		var file FileInfo
		var importsJSON string
		var symbolsJSON string
		var isTest int
		var isRoute int
		var isConfig int
		if err := rows.Scan(
			&file.Path,
			&file.Language,
			&file.SizeBytes,
			&file.ModTimeUnixNano,
			&file.EstimatedTokens,
			&file.Package,
			&importsJSON,
			&symbolsJSON,
			&isTest,
			&isRoute,
			&isConfig,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(importsJSON), &file.Imports); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(symbolsJSON), &file.Symbols); err != nil {
			return nil, err
		}
		file.IsTest = isTest != 0
		file.IsRoute = isRoute != 0
		file.IsConfig = isConfig != 0
		out[file.Path] = file
	}
	return out, rows.Err()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
