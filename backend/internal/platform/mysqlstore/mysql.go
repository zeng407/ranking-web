package mysqlstore

import (
	"database/sql"
	"errors"
	"net"
	"strconv"
	"time"

	"2pick.app/backend/internal/config"
	"github.com/go-sql-driver/mysql"
)

// Option adjusts the driver configuration for callers whose workload does not
// fit the request-serving defaults.
type Option func(*mysql.Config)

// WithStatementTimeouts overrides the per-read and per-write socket timeouts.
//
// The defaults are sized for serving HTTP requests, where a query that has not
// answered in ten seconds is already a failure. Schema work is the opposite:
// a GROUP BY or index build over a multi-GB table legitimately runs for minutes,
// and the ten second read timeout kills the connection mid-statement while the
// server keeps executing, which looks like a silent no-op.
func WithStatementTimeouts(read, write time.Duration) Option {
	return func(driverConfig *mysql.Config) {
		driverConfig.ReadTimeout = read
		driverConfig.WriteTimeout = write
	}
}

func Open(configuration config.DatabaseConfig, options ...Option) (*sql.DB, error) {
	driverConfig := mysql.Config{
		User:                 configuration.User,
		Passwd:               configuration.Password,
		Net:                  "tcp",
		Addr:                 net.JoinHostPort(configuration.Host, strconv.Itoa(configuration.Port)),
		DBName:               configuration.Name,
		Collation:            "utf8mb4_unicode_ci",
		ParseTime:            true,
		Loc:                  time.FixedZone("Asia/Taipei", 8*60*60),
		Timeout:              3 * time.Second,
		ReadTimeout:          10 * time.Second,
		WriteTimeout:         10 * time.Second,
		AllowNativePasswords: true,
	}
	for _, option := range options {
		option(&driverConfig)
	}
	connector, err := mysql.NewConnector(&driverConfig)
	if err != nil {
		return nil, err
	}

	database := sql.OpenDB(connector)
	database.SetMaxOpenConns(configuration.MaxOpenConns)
	database.SetMaxIdleConns(configuration.MaxIdleConns)
	database.SetConnMaxLifetime(configuration.ConnMaxLifetime)
	database.SetConnMaxIdleTime(configuration.ConnMaxIdleTime)
	return database, nil
}

// DuplicateEntryErrorNumber is MySQL's error for a unique index rejecting a write.
const DuplicateEntryErrorNumber = 1062

// IsDuplicateKey reports whether a write lost a race to a unique index.
//
// Matched on the driver's error number rather than the message, which is localised and
// names the key. Shared rather than reimplemented per package: several stores now depend
// on "the index arbitrated, re-read the winner's row" being classified the same way, and
// two copies of this is how one of them ends up treating a lost race as a 500.
func IsDuplicateKey(err error) bool {
	var mysqlError *mysql.MySQLError
	if errors.As(err, &mysqlError) {
		return mysqlError.Number == DuplicateEntryErrorNumber
	}
	return false
}
