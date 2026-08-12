// Command migrate applies the Go-owned schema history.
//
// It is a separate binary on purpose: the api, worker and scheduler must never
// migrate a database as a side effect of starting up, because several replicas
// start at once.
//
// Subcommands:
//
//	status    show which versions are applied
//	up        apply all pending versions
//	up-by-one apply the next pending version
//	down      roll back the most recent version
//	version   print the current version
//	baseline  stamp the baseline as applied WITHOUT running it, for a database
//	          that already has the schema
//	stamp N   record version N as applied WITHOUT running it, for a change made
//	          out of band with gh-ost or pt-online-schema-change
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"2pick.app/backend/internal/config"
	"2pick.app/backend/internal/platform/mysqlstore"
	"2pick.app/backend/migrations"
	"github.com/pressly/goose/v3"
)

// laravelTableName is the history goose must never touch.
const laravelTableName = "migrations"

// schemaProbeTable is a table that exists only once the baseline has been
// applied, used to tell a fresh database from an established one.
const schemaProbeTable = "posts"

// migrationStatementTimeout bounds one migration statement. It is generous
// because index builds over the four large tables legitimately take minutes, but
// finite so a statement blocked on a metadata lock eventually fails instead of
// hanging the deploy forever.
const migrationStatementTimeout = time.Hour

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(context.Background(), logger, os.Args[1:]); err != nil {
		logger.Error("migrate_failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger, args []string) error {
	if len(args) == 0 {
		return errors.New("a subcommand is required: status, up, up-by-one, down, version or baseline")
	}
	command := args[0]

	if migrations.TableName == laravelTableName {
		return fmt.Errorf("refusing to run: the goose version table is %q, which belongs to Laravel", laravelTableName)
	}

	configuration, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	if !configuration.Database.Enabled() {
		return errors.New("DB_HOST is required")
	}

	// Schema work runs far longer than a request. The default ten second read
	// timeout kills the connection mid-statement while MySQL keeps executing,
	// which surfaces as "invalid connection" on the next statement and looks like
	// the migration silently did nothing.
	database, err := mysqlstore.Open(configuration.Database,
		mysqlstore.WithStatementTimeouts(migrationStatementTimeout, migrationStatementTimeout))
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer database.Close()

	// A single connection keeps every statement of a NO TRANSACTION migration on
	// the same session, which session-scoped settings and staging tables rely on.
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)

	pingContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := database.PingContext(pingContext); err != nil {
		return fmt.Errorf("database unreachable: %w", err)
	}

	goose.SetBaseFS(migrations.FS)
	goose.SetTableName(migrations.TableName)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}

	logger.Info("migrate_starting",
		"command", command,
		"database", configuration.Database.Name,
		"version_table", migrations.TableName,
	)

	switch command {
	case "baseline":
		return baseline(ctx, logger, database, configuration.Database.Name)
	case "status":
		return goose.StatusContext(ctx, database, ".")
	case "up":
		return goose.UpContext(ctx, database, ".")
	case "up-by-one":
		return goose.UpByOneContext(ctx, database, ".")
	case "down":
		return goose.DownContext(ctx, database, ".")
	case "version":
		version, err := goose.GetDBVersionContext(ctx, database)
		if err != nil {
			return err
		}
		logger.Info("migrate_version", "version", version)
		return nil
	case "stamp":
		if len(args) < 2 {
			return errors.New("stamp requires a version, e.g. `migrate stamp 3`")
		}
		return stamp(ctx, logger, database, args[1])
	default:
		return fmt.Errorf("unknown subcommand %q", command)
	}
}

// baseline records the baseline version as applied on a database that already
// has the schema. Running the baseline SQL there would fail on the first
// existing table, and forcing it through would be worse.
func baseline(ctx context.Context, logger *slog.Logger, database *sql.DB, schema string) error {
	established, err := schemaExists(ctx, database, schema, schemaProbeTable)
	if err != nil {
		return err
	}
	if !established {
		return fmt.Errorf(
			"table %q does not exist, so this looks like a fresh database: run `up` to apply the baseline instead of stamping it",
			schemaProbeTable,
		)
	}

	// EnsureDBVersion creates the version table if it is missing and reports the
	// current version, which is 0 on a database goose has not tracked before.
	current, err := goose.EnsureDBVersionContext(ctx, database)
	if err != nil {
		return fmt.Errorf("ensure version table: %w", err)
	}
	if current >= migrations.BaselineVersion {
		logger.Info("migrate_baseline_already_applied", "version", current)
		return nil
	}

	if _, err := database.ExecContext(ctx,
		fmt.Sprintf("INSERT INTO `%s` (version_id, is_applied, tstamp) VALUES (?, ?, NOW())", migrations.TableName),
		migrations.BaselineVersion, true,
	); err != nil {
		return fmt.Errorf("stamp baseline version: %w", err)
	}

	logger.Info("migrate_baseline_stamped",
		"version", migrations.BaselineVersion,
		"note", "the existing schema is now the recorded starting point; new migrations apply forward from here",
	)
	return nil
}

// stamp records a version as applied without running it.
//
// This is how a change applied out of band is reconciled with the history. The
// intended case is the large tables: adding an index to `ranks`,
// `game_1v1_rounds`, `game_elements` or `rank_report_histories` should be done
// with gh-ost or pt-online-schema-change so it can be throttled and paused, and
// then stamped here so `up` does not try to apply it again.
func stamp(ctx context.Context, logger *slog.Logger, database *sql.DB, rawVersion string) error {
	version, err := strconv.ParseInt(strings.TrimSpace(rawVersion), 10, 64)
	if err != nil || version < 1 {
		return fmt.Errorf("stamp: version must be a positive integer, got %q", rawVersion)
	}

	current, err := goose.EnsureDBVersionContext(ctx, database)
	if err != nil {
		return fmt.Errorf("stamp: ensure version table: %w", err)
	}

	var exists int
	err = database.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM `%s` WHERE version_id = ?", migrations.TableName),
		version,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("stamp: check version %d: %w", version, err)
	}
	if exists > 0 {
		logger.Info("migrate_stamp_already_recorded", "version", version, "current", current)
		return nil
	}

	if _, err := database.ExecContext(ctx,
		fmt.Sprintf("INSERT INTO `%s` (version_id, is_applied, tstamp) VALUES (?, ?, NOW())", migrations.TableName),
		version, true,
	); err != nil {
		return fmt.Errorf("stamp: record version %d: %w", version, err)
	}

	logger.Info("migrate_stamped",
		"version", version,
		"note", "recorded as applied without running; the change must already exist in the schema",
	)
	return nil
}

func schemaExists(ctx context.Context, database *sql.DB, schema, table string) (bool, error) {
	var count int
	err := database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = ?`,
		schema, table,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("probe for table %q: %w", table, err)
	}
	return count > 0, nil
}
