package postaccess

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// go test ./... runs packages concurrently, and every one of them that touches MySQL
// touches the SAME database. A test that counts every live post therefore races the
// fixtures in internal/authoring and internal/ingest, which create posts and delete them
// again: their rows are real, live and indistinguishable from anyone else's while they
// exist.
//
// It failed exactly that way — "counted 5580 posts, want every live post (5576)" — once
// the ingest package started creating posts too, and passed when run on its own.
//
// A MySQL advisory lock is what serialises them, because the contending tests are in
// different packages and no Go-level mutex spans those. Every fixture that adds or
// removes posts takes the same lock; see the matching helper in the other two packages.
const sharedPostsLock = "2pick_go_tests_posts"

// lockSharedPosts holds the lock for the duration of one test.
func lockSharedPosts(t *testing.T, database *sql.DB) {
	t.Helper()
	ctx := context.Background()

	// A dedicated connection, not the pool: GET_LOCK belongs to the connection that took
	// it, and a pooled release would run somewhere else and quietly do nothing.
	conn, err := database.Conn(ctx)
	if err != nil {
		t.Fatalf("connection for the shared-posts lock: %v", err)
	}

	// WAITED FOR IN SHORT HOPS, NOT ONE LONG ONE. The driver is opened with a ten second
	// read timeout, and a GET_LOCK that blocks past it takes the connection down with it
	// — "invalid connection" after exactly ten seconds, which is what a single
	// GET_LOCK(?, 60) produced here. Each attempt stays well inside that; the deadline
	// below is what actually bounds the wait.
	deadline := time.Now().Add(2 * time.Minute)
	for {
		var acquired sql.NullInt64
		if err := conn.QueryRowContext(ctx,
			`SELECT GET_LOCK(?, 3)`, sharedPostsLock).Scan(&acquired); err != nil {
			conn.Close()
			t.Fatalf("take the shared-posts lock: %v", err)
		}
		if acquired.Valid && acquired.Int64 == 1 {
			break
		}
		if time.Now().After(deadline) {
			conn.Close()
			t.Fatal("timed out waiting for the shared-posts lock")
		}
	}

	t.Cleanup(func() {
		if _, err := conn.ExecContext(ctx, `SELECT RELEASE_LOCK(?)`, sharedPostsLock); err != nil {
			t.Errorf("release the shared-posts lock: %v", err)
		}
		conn.Close()
	})
}
