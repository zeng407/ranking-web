package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"2pick.app/backend/internal/gameplay"
	"2pick.app/backend/internal/postaccess"
)

type fakeGameplay struct {
	createInput  gameplay.CreateInput
	batchInput   gameplay.BatchInput
	batchSerial  string
	resultSerial string
	err          error
	// batchResult overrides what SubmitVotes reports, so a test can drive the
	// game-completed branch.
	batchResult *gameplay.BatchResult
	// caller records what the handler decided the request had proved, which is the
	// whole of the access rule as far as these endpoints are concerned.
	caller postaccess.Caller
}

func (repository *fakeGameplay) Definition(_ context.Context, _ string, caller postaccess.Caller) (gameplay.Definition, error) {
	repository.caller = caller
	return gameplay.Definition{Title: "Test", Serial: "post-1", ElementsCount: 32, MaxElements: 32}, repository.err
}

func (repository *fakeGameplay) Create(_ context.Context, input gameplay.CreateInput) (gameplay.Session, error) {
	repository.createInput = input
	repository.caller = input.Caller
	return gameplay.Session{GameSerial: "game-1", ServerVoteCount: 0}, repository.err
}

func (repository *fakeGameplay) Resume(_ context.Context, _ string, caller postaccess.Caller) (gameplay.Session, error) {
	repository.caller = caller
	return gameplay.Session{
		GameSerial: "game-1", ServerVoteCount: 4,
		Definition: gameplay.Definition{Serial: "post-1"},
	}, repository.err
}

func (repository *fakeGameplay) SubmitVotes(_ context.Context, serial string, input gameplay.BatchInput) (gameplay.BatchResult, error) {
	repository.batchSerial = serial
	repository.batchInput = input
	repository.caller = input.Caller
	if repository.batchResult != nil {
		return *repository.batchResult, repository.err
	}
	return gameplay.BatchResult{Status: "processing", ServerVoteCount: len(input.Votes)}, repository.err
}

func (repository *fakeGameplay) Result(_ context.Context, serial string, caller postaccess.Caller) (gameplay.GameResult, error) {
	repository.resultSerial = serial
	repository.caller = caller
	globalRank := int64(7)
	return gameplay.GameResult{
		GameSerial: "game-1",
		PostSerial: "post-1",
		Items: []gameplay.GameResultItem{{
			Rank: 1, WinCount: 5, GlobalRank: &globalRank,
			Element: gameplay.Element{ID: 11, Title: "Winner", Type: "image"},
		}},
	}, repository.err
}

func gameplayTestHandler(repository gameplay.Repository) http.Handler {
	return New(Options{
		Environment: "test",
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Gameplay:    repository,
	})
}

func TestGameDefinitionIsPubliclyCacheable(t *testing.T) {
	response := httptest.NewRecorder()
	gameplayTestHandler(&fakeGameplay{}).ServeHTTP(
		response, httptest.NewRequest(http.MethodGet, "/api/v1/game-posts/post-1", nil),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cloudflare-CDN-Cache-Control") != publicEdgeCache {
		t.Fatalf("Cloudflare-CDN-Cache-Control = %q", response.Header().Get("Cloudflare-CDN-Cache-Control"))
	}
}

func TestCreateGameIsPrivate(t *testing.T) {
	repository := &fakeGameplay{}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/games", strings.NewReader(`{"post_serial":"post-1","element_count":16}`))
	request.Header.Set("Content-Type", "application/json")
	gameplayTestHandler(repository).ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
	if repository.createInput.PostSerial != "post-1" || repository.createInput.ElementCount != 16 {
		t.Fatalf("input = %#v", repository.createInput)
	}
}

func TestResumeGameIsPrivate(t *testing.T) {
	response := httptest.NewRecorder()
	gameplayTestHandler(&fakeGameplay{}).ServeHTTP(
		response, httptest.NewRequest(http.MethodGet, "/api/v1/games/game-1/elements", nil),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
}

func TestCompletedGameResultIncludesPersonalAndGlobalRanks(t *testing.T) {
	repository := &fakeGameplay{}
	response := httptest.NewRecorder()
	gameplayTestHandler(repository).ServeHTTP(
		response, httptest.NewRequest(http.MethodGet, "/api/v1/games/game-1/result", nil),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
	if repository.resultSerial != "game-1" {
		t.Fatalf("result serial = %q", repository.resultSerial)
	}
	var payload struct {
		Data gameplay.GameResult `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.PostSerial != "post-1" || len(payload.Data.Items) != 1 {
		t.Fatalf("result = %#v", payload.Data)
	}
	if payload.Data.Items[0].GlobalRank == nil || *payload.Data.Items[0].GlobalRank != 7 {
		t.Fatalf("item = %#v", payload.Data.Items[0])
	}
}

func TestBatchVotesDeduplicatesExactRetries(t *testing.T) {
	repository := &fakeGameplay{}
	body := `{"expected_vote_count":10,"anonymous_id":"anon","votes":[{"winner_id":1,"loser_id":2},{"winner_id":1,"loser_id":2},{"winner_id":3,"loser_id":4}]}`
	response := httptest.NewRecorder()
	gameplayTestHandler(repository).ServeHTTP(
		response, httptest.NewRequest(http.MethodPost, "/api/v1/games/game-1/votes/batch", strings.NewReader(body)),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if repository.batchSerial != "game-1" || repository.batchInput.ExpectedVoteCount != 10 || len(repository.batchInput.Votes) != 2 {
		t.Fatalf("batch serial = %q, input = %#v", repository.batchSerial, repository.batchInput)
	}
}

func TestBatchConflictExposesServerRevision(t *testing.T) {
	repository := &fakeGameplay{err: &gameplay.ConflictError{Reason: "revision_mismatch", ServerVoteCount: 12}}
	response := httptest.NewRecorder()
	gameplayTestHandler(repository).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodPost, "/api/v1/games/game-1/votes/batch", strings.NewReader(
			`{"expected_vote_count":10,"votes":[{"winner_id":1,"loser_id":2}]}`,
		)),
	)
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			Reason          string `json:"reason"`
			ServerVoteCount int    `json:"server_vote_count"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Reason != "revision_mismatch" || payload.Data.ServerVoteCount != 12 {
		t.Fatalf("data = %#v", payload.Data)
	}
}

func TestBatchVotesRequiresRevision(t *testing.T) {
	response := httptest.NewRecorder()
	gameplayTestHandler(&fakeGameplay{}).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodPost, "/api/v1/games/game-1/votes/batch", strings.NewReader(
			`{"votes":[{"winner_id":1,"loser_id":2}]}`,
		)),
	)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

// fakeFreshness records what the completion path flagged.
type fakeFreshness struct {
	set   []int64
	err   error
	calls int
}

func (store *fakeFreshness) NeedsRebuild(context.Context, int64) (bool, error) { return false, nil }
func (store *fakeFreshness) Clear(context.Context, int64) error                { return nil }
func (store *fakeFreshness) Set(_ context.Context, postID int64) error {
	store.calls++
	if store.err != nil {
		return store.err
	}
	store.set = append(store.set, postID)
	return nil
}

func votesRequest() *http.Request {
	body := `{"expected_vote_count":0,"votes":[{"winner_id":11,"loser_id":12}],"anonymous_id":"anon"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/games/game-1/votes/batch", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func freshnessHandler(repository gameplay.Repository, store *fakeFreshness) http.Handler {
	return New(Options{
		Environment:   "test",
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Gameplay:      repository,
		RankFreshness: store,
	})
}

// Finishing a game must flag the post, which is what App\Listeners\UpdatePostRank
// does on GameComplete. Without it the daily rank history sweep never sees the post.
func TestFinishingAGameFlagsThePostRanks(t *testing.T) {
	store := &fakeFreshness{}
	repository := &fakeGameplay{batchResult: &gameplay.BatchResult{
		Status: "end", ServerVoteCount: 31, Complete: true, JustCompleted: true, PostID: 4242,
	}}

	response := httptest.NewRecorder()
	freshnessHandler(repository, store).ServeHTTP(response, votesRequest())

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if len(store.set) != 1 || store.set[0] != 4242 {
		t.Fatalf("flagged %v, want [4242]", store.set)
	}
}

// A vote batch that does not finish the game must not flag anything: the sweep would
// then rebuild history for every post that was merely played, which is a full
// aggregation over 50.7M rows per post.
func TestAnUnfinishedGameDoesNotFlagThePost(t *testing.T) {
	store := &fakeFreshness{}
	repository := &fakeGameplay{batchResult: &gameplay.BatchResult{
		Status: "processing", ServerVoteCount: 4, Complete: false, JustCompleted: false, PostID: 4242,
	}}

	response := httptest.NewRecorder()
	freshnessHandler(repository, store).ServeHTTP(response, votesRequest())

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if store.calls != 0 {
		t.Fatalf("flagged %d times for an unfinished game, want 0", store.calls)
	}
}

// A client retrying against an already-finished game reports Complete but not
// JustCompleted. Re-flagging there would let a retry loop trigger the rebuild
// repeatedly.
func TestRetryingAFinishedGameDoesNotReflag(t *testing.T) {
	store := &fakeFreshness{}
	repository := &fakeGameplay{batchResult: &gameplay.BatchResult{
		Status: "end", ServerVoteCount: 31, Complete: true, JustCompleted: false, PostID: 4242,
	}}

	response := httptest.NewRecorder()
	freshnessHandler(repository, store).ServeHTTP(response, votesRequest())

	if store.calls != 0 {
		t.Fatalf("flagged %d times on a retry, want 0", store.calls)
	}
}

// The votes are already committed when the flag is written, so a Redis failure must
// not turn a successful game into an error the client would retry.
func TestAFreshnessFailureDoesNotFailTheRequest(t *testing.T) {
	store := &fakeFreshness{err: errors.New("redis unreachable")}
	repository := &fakeGameplay{batchResult: &gameplay.BatchResult{
		Status: "end", ServerVoteCount: 31, Complete: true, JustCompleted: true, PostID: 4242,
	}}

	response := httptest.NewRecorder()
	freshnessHandler(repository, store).ServeHTTP(response, votesRequest())

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 despite the freshness failure", response.Code)
	}
	if store.calls != 1 {
		t.Fatalf("attempted %d writes, want 1", store.calls)
	}
}

// The API must keep serving when no store is configured at all, which is how it ran
// before the flag existed.
func TestCompletionWithoutAFreshnessStoreStillSucceeds(t *testing.T) {
	repository := &fakeGameplay{batchResult: &gameplay.BatchResult{
		Status: "end", ServerVoteCount: 31, Complete: true, JustCompleted: true, PostID: 4242,
	}}

	response := httptest.NewRecorder()
	gameplayTestHandler(repository).ServeHTTP(response, votesRequest())

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with no freshness store", response.Code)
	}
}

// JustCompleted and PostID are internal signals for the transport layer; leaking
// them would add fields the browser never asked for.
func TestCompletionSignalsAreNotSerialised(t *testing.T) {
	repository := &fakeGameplay{batchResult: &gameplay.BatchResult{
		Status: "end", ServerVoteCount: 31, Complete: true, JustCompleted: true, PostID: 4242,
	}}

	response := httptest.NewRecorder()
	freshnessHandler(repository, &fakeFreshness{}).ServeHTTP(response, votesRequest())

	body := response.Body.String()
	for _, leaked := range []string{"just_completed", "JustCompleted", "post_id", "PostID", "4242"} {
		if strings.Contains(body, leaked) {
			t.Errorf("response leaks %q: %s", leaked, body)
		}
	}
	if !strings.Contains(body, `"complete":true`) {
		t.Errorf("the public complete flag is missing: %s", body)
	}
}

// Starting a game on an adult post without an account is 401, so the browser can send the
// visitor to the sign-in page instead of showing the door-code box a 404 would imply.
func TestCreateGameAsksAnAdultPostVisitorToSignIn(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/games",
		strings.NewReader(`{"post_serial":"post-1","element_count":16}`))
	request.Header.Set("Content-Type", "application/json")
	gameplayTestHandler(&fakeGameplay{err: postaccess.ErrSignInRequired}).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
	}
}
