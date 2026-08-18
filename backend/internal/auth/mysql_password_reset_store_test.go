package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// The reset store against the real server. What is only true here is the claim: one
// UPDATE that checks used_at and expires_at in its own WHERE clause, read back through
// RowsAffected. The in-memory store in password_reset_test.go imitates that, so this file
// is what keeps the imitation honest — and it is the only place where two requests can
// genuinely race for the same token.
//
// Skipped unless MYSQL_TEST_HOST is set, as with the other store tests.

func newResetFixture(t *testing.T) (*accountFixture, *MySQLPasswordResetStore, context.Context) {
	t.Helper()
	// The go_password_resets rows go with the user: the foreign key cascades, so the
	// fixture's own cleanup is enough.
	fixture, ctx := newAccountFixture(t)
	return fixture, NewMySQLPasswordResetStore(fixture.database), ctx
}

func (fixture *accountFixture) mustCreateReset(
	t *testing.T, ctx context.Context, store *MySQLPasswordResetStore,
	userID int64, tokenHash string, requestedAt, expiresAt time.Time,
) {
	t.Helper()
	if err := store.Create(ctx, NewPasswordReset{
		UserID: userID, TokenHash: tokenHash,
		RequestedAt: requestedAt, ExpiresAt: expiresAt,
		RequestedIP: "203.0.113.7",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestMySQLPasswordResetStoreClaimsALiveTokenExactlyOnce(t *testing.T) {
	fixture, store, ctx := newResetFixture(t)
	userID := fixture.createUser(t, ctx, "claim", "$2y$10$hash", nil)

	now := time.Now().UTC().Truncate(time.Second)
	hash := HashToken("the-live-token")
	fixture.mustCreateReset(t, ctx, store, userID, hash, now, now.Add(ResetTokenTTL))

	claimed, err := store.Consume(ctx, hash, now)
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if claimed != userID {
		t.Errorf("user id = %d, want %d", claimed, userID)
	}

	// THE SECOND CLAIM MUST FAIL. used_at is in the WHERE clause, so the row is no
	// longer claimable — a store that checked used_at with a separate SELECT would let
	// this through.
	if _, err := store.Consume(ctx, hash, now); !errors.Is(err, ErrResetTokenInvalid) {
		t.Errorf("the second Consume() error = %v, want ErrResetTokenInvalid", err)
	}
}

// Two requests arriving together with the same token — the case the single-statement
// claim exists for. Exactly one may win; two winners means two people just set the
// password on one account.
func TestMySQLPasswordResetStoreLetsOnlyOneOfTwoRacingClaimsWin(t *testing.T) {
	fixture, store, ctx := newResetFixture(t)
	userID := fixture.createUser(t, ctx, "race", "$2y$10$hash", nil)

	now := time.Now().UTC().Truncate(time.Second)
	hash := HashToken("the-contested-token")
	fixture.mustCreateReset(t, ctx, store, userID, hash, now, now.Add(ResetTokenTTL))

	var (
		start   sync.WaitGroup
		done    sync.WaitGroup
		mu      sync.Mutex
		winners int
		other   []error
	)
	start.Add(1)
	for range 2 {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			_, err := store.Consume(ctx, hash, now)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				winners++
			case errors.Is(err, ErrResetTokenInvalid):
			default:
				other = append(other, err)
			}
		}()
	}
	start.Done()
	done.Wait()

	if winners != 1 {
		t.Errorf("claims that succeeded = %d, want exactly 1", winners)
	}
	for _, err := range other {
		t.Errorf("unexpected Consume() error = %v", err)
	}
}

func TestMySQLPasswordResetStoreRefusesAnExpiredToken(t *testing.T) {
	fixture, store, ctx := newResetFixture(t)
	userID := fixture.createUser(t, ctx, "expired", "$2y$10$hash", nil)

	now := time.Now().UTC().Truncate(time.Second)
	hash := HashToken("the-stale-token")
	fixture.mustCreateReset(t, ctx, store, userID, hash, now.Add(-2*ResetTokenTTL), now.Add(-time.Minute))

	if _, err := store.Consume(ctx, hash, now); !errors.Is(err, ErrResetTokenInvalid) {
		t.Errorf("Consume() error = %v, want ErrResetTokenInvalid", err)
	}
}

func TestMySQLPasswordResetStoreRefusesATokenItNeverIssued(t *testing.T) {
	_, store, ctx := newResetFixture(t)

	if _, err := store.Consume(ctx, HashToken("nobody-issued-this"), time.Now().UTC()); !errors.Is(
		err, ErrResetTokenInvalid) {
		t.Errorf("Consume() error = %v, want ErrResetTokenInvalid", err)
	}
}

func TestMySQLPasswordResetStoreReportsTheNewestRequest(t *testing.T) {
	fixture, store, ctx := newResetFixture(t)
	userID := fixture.createUser(t, ctx, "throttle", "$2y$10$hash", nil)

	if _, found, err := store.LastRequestedAt(ctx, userID); err != nil {
		t.Fatalf("LastRequestedAt() error = %v", err)
	} else if found {
		t.Error("an account that never asked reports a request")
	}

	now := time.Now().UTC().Truncate(time.Second)
	older := now.Add(-10 * time.Minute)
	fixture.mustCreateReset(t, ctx, store, userID, HashToken("older"), older, older.Add(ResetTokenTTL))
	fixture.mustCreateReset(t, ctx, store, userID, HashToken("newer"), now, now.Add(ResetTokenTTL))

	lastRequested, found, err := store.LastRequestedAt(ctx, userID)
	if err != nil {
		t.Fatalf("LastRequestedAt() error = %v", err)
	}
	if !found {
		t.Fatal("the account's requests were not found")
	}
	// The throttle compares against this, so the older row winning would let an account
	// ask for a mail every second.
	if !lastRequested.Equal(now) {
		t.Errorf("last requested = %s, want %s", lastRequested, now)
	}
}
