// Package migrations holds the Go-owned schema history.
//
// This history is independent of Laravel's `migrations` table. Nothing here
// reads or writes that table, and the two must never manage the same schema
// change, because that table disagrees with the repository in both environments
// measured so far:
//
//	production dump (2026-07-29): 68 rows, 66 migration files, 2 missing sources
//	local dev rk-db (2026-08-04): 62 rows, 66 migration files, and one row,
//	  2023_02_05_200639_add_original_url_into_elements, with no file on disk
//
// So it cannot serve as a source of truth for what the schema actually is.
//
// Anything that alters game_1v1_rounds, game_elements, ranks or
// rank_report_histories must not be expressed as a plain ALTER TABLE here. Those
// four tables hold about 93% of the data, and a naive ALTER locks them for
// hours; use gh-ost or pt-online-schema-change and record the operation as a
// no-op migration.
package migrations

import "embed"

// FS carries the migration files into the binary so a release image needs no
// accompanying SQL on disk.
//
//go:embed *.sql
var FS embed.FS

// BaselineVersion is the version representing the pre-existing schema. On a
// database that already has those tables it is stamped as applied rather than
// executed.
const BaselineVersion int64 = 1

// TableName is the goose version table. It is explicitly not "migrations", which
// belongs to Laravel.
const TableName = "go_schema_migrations"
