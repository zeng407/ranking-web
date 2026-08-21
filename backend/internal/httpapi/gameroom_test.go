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

	"2pick.app/backend/internal/auth"
	"2pick.app/backend/internal/gameroom"
)

// fakeGameRoom stands in for both halves of the room. The domain rules are covered in
// internal/gameroom; what matters here is status codes, what identity reaches the service,
// and what the response does not contain.
type fakeGameRoom struct {
	room        gameroom.Room
	gameSerial  string
	created     bool
	participant gameroom.Participant
	votes       gameroom.VoteTally
	votesFound  bool
	latestBet   gameroom.PlacedBet
	betFound    bool
	board       gameroom.Leaderboard
	roomFound   bool

	ensureErr error
	joinErr   error
	betErr    error
	renameErr error
	readErr   error

	joinCalls         int
	betCalls          int
	renameCalls       int
	rebindCalls       int
	lastRebindFrom    string
	lastRebindTo      string
	rebindErr         error
	lastAnonymousID   string
	lastLocale        string
	lastUserID        *int64
	lastBet           gameroom.PlacedBet
	lastBetGameSerial string
	lastNickname      string
	lastOnScreen      []int64
}

func newFakeGameRoom() *fakeGameRoom {
	return &fakeGameRoom{
		room:       gameroom.Room{ID: 5, Serial: "abcdefgh"},
		gameSerial: "game-serial",
		roomFound:  true,
		participant: gameroom.Participant{
			ID: 7, RoomID: 5, AnonymousID: "browser-a", Nickname: "路人",
			Score: 1000, Rank: 3, AccuracyHundredths: 6349, TotalPlayed: 10,
			TotalCorrect: 6, Combo: 2,
		},
		board: gameroom.Leaderboard{},
	}
}

func (fake *fakeGameRoom) EnsureRoom(
	_ context.Context, _ string, onScreen []int64,
) (gameroom.Room, bool, error) {
	fake.lastOnScreen = onScreen
	if fake.ensureErr != nil {
		return gameroom.Room{}, false, fake.ensureErr
	}
	return fake.room, fake.created, nil
}

func (fake *fakeGameRoom) Join(
	_ context.Context, _ int64, anonymousID string, userID *int64, locale string,
) (gameroom.Participant, error) {
	fake.joinCalls++
	fake.lastAnonymousID, fake.lastUserID, fake.lastLocale = anonymousID, userID, locale
	if fake.joinErr != nil {
		return gameroom.Participant{}, fake.joinErr
	}
	return fake.participant, nil
}

func (fake *fakeGameRoom) BetOnCurrentRound(
	_ context.Context, _ int64, _ gameroom.Participant, gameSerial string, winnerID, loserID int64,
) error {
	fake.betCalls++
	fake.lastBetGameSerial = gameSerial
	fake.lastBet = gameroom.PlacedBet{WinnerID: winnerID, LoserID: loserID}
	return fake.betErr
}

func (fake *fakeGameRoom) Rebind(
	_ context.Context, _, fromGameSerial, toGameSerial string, onScreen []int64,
) (gameroom.Room, error) {
	fake.rebindCalls++
	fake.lastRebindFrom, fake.lastRebindTo = fromGameSerial, toGameSerial
	fake.lastOnScreen = onScreen
	if fake.rebindErr != nil {
		return gameroom.Room{}, fake.rebindErr
	}
	return fake.room, nil
}

func (fake *fakeGameRoom) Rename(_ context.Context, _ gameroom.Participant, nickname string) error {
	fake.renameCalls++
	fake.lastNickname = nickname
	return fake.renameErr
}

func (fake *fakeGameRoom) RoomBySerialWithGame(
	_ context.Context, _ string,
) (gameroom.Room, string, bool, error) {
	if fake.readErr != nil {
		return gameroom.Room{}, "", false, fake.readErr
	}
	return fake.room, fake.gameSerial, fake.roomFound, nil
}

func (fake *fakeGameRoom) CurrentVotes(_ context.Context, _ int64, _ string) (gameroom.VoteTally, bool, error) {
	return fake.votes, fake.votesFound, fake.readErr
}

func (fake *fakeGameRoom) LatestBet(_ context.Context, _ int64) (gameroom.PlacedBet, bool, error) {
	return fake.latestBet, fake.betFound, fake.readErr
}

func (fake *fakeGameRoom) Leaderboard(_ context.Context, _ int64) (gameroom.Leaderboard, error) {
	return fake.board, fake.readErr
}

func gameRoomHandler(fake *fakeGameRoom) http.Handler {
	return New(Options{
		Environment:    "test",
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		AllowedOrigins: []string{"http://localhost:4173"},
		GameRooms:      fake,
		GameRoomReader: fake,
		GameRoomBoard:  fake,
		AuthVerifier:   staticTokenVerifier{identity: auth.Identity{Subject: "42"}},
	})
}

func decodeData(t *testing.T, response *httptest.ResponseRecorder, into any) {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v; body = %s", err, response.Body.String())
	}
	if err := json.Unmarshal(envelope.Data, into); err != nil {
		t.Fatalf("decode data: %v; body = %s", err, response.Body.String())
	}
}

func TestCreateGameRoomReportsWhetherItOpenedOne(t *testing.T) {
	for _, testCase := range []struct {
		created bool
		want    int
	}{{true, http.StatusCreated}, {false, http.StatusOK}} {
		fake := newFakeGameRoom()
		fake.created = testCase.created
		response := httptest.NewRecorder()
		gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodPost,
			"/api/v1/game-rooms", strings.NewReader(`{"game_serial":"game-serial"}`)))

		if response.Code != testCase.want {
			t.Errorf("created = %v: status = %d, want %d", testCase.created, response.Code, testCase.want)
		}
		var body struct {
			Serial string `json:"serial"`
		}
		decodeData(t, response, &body)
		if body.Serial != "abcdefgh" {
			t.Errorf("serial = %q", body.Serial)
		}
	}
}

// The pair on screen travels with the request, because the server cannot know it: the Go
// client plays its bracket locally rather than asking for each next pair.
func TestCreateGameRoomForwardsThePairOnScreen(t *testing.T) {
	fake := newFakeGameRoom()
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodPost,
		"/api/v1/game-rooms",
		strings.NewReader(`{"game_serial":"game-serial","current_candidates":[11,22]}`)))

	if response.Code != http.StatusOK && response.Code != http.StatusCreated {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if len(fake.lastOnScreen) != 2 || fake.lastOnScreen[0] != 11 || fake.lastOnScreen[1] != 22 {
		t.Errorf("on-screen pair = %v, want [11 22]", fake.lastOnScreen)
	}
}

func TestCreateGameRoomRequiresAGameSerial(t *testing.T) {
	fake := newFakeGameRoom()
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodPost,
		"/api/v1/game-rooms", strings.NewReader(`{"game_serial":"  "}`)))

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", response.Code)
	}
}

func TestCreateGameRoomReportsAnUnknownGame(t *testing.T) {
	fake := newFakeGameRoom()
	fake.ensureErr = gameroom.ErrGameNotFound
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodPost,
		"/api/v1/game-rooms", strings.NewReader(`{"game_serial":"nope"}`)))

	if response.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", response.Code)
	}
}

/**
 * The restart case. The host's game serial changes and the room has to follow, keeping the
 * serial already handed out on invite links and QR codes.
 */
func TestRebindGameRoomMovesTheRoomAndForwardsThePair(t *testing.T) {
	fake := newFakeGameRoom()
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodPut,
		"/api/v1/game-rooms/abcdefgh/game",
		strings.NewReader(`{"from_game_serial":"old-game","game_serial":"new-game","current_candidates":[11,22]}`)))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	if fake.rebindCalls != 1 || fake.lastRebindFrom != "old-game" || fake.lastRebindTo != "new-game" {
		t.Errorf("rebind calls = %d, %q -> %q; want one call old-game -> new-game",
			fake.rebindCalls, fake.lastRebindFrom, fake.lastRebindTo)
	}
	if len(fake.lastOnScreen) != 2 || fake.lastOnScreen[0] != 11 || fake.lastOnScreen[1] != 22 {
		t.Errorf("on-screen pair = %v, want [11 22]", fake.lastOnScreen)
	}

	var body struct {
		Serial     string `json:"serial"`
		GameSerial string `json:"game_serial"`
	}
	decodeData(t, response, &body)
	if body.Serial != "abcdefgh" || body.GameSerial != "new-game" {
		t.Errorf("body = %+v, want the same room on the new game", body)
	}
}

// Both serials are required: without the source there is nothing to check the move against,
// and any caller could drag the room onto a game of its choosing.
func TestRebindGameRoomRequiresBothGameSerials(t *testing.T) {
	for name, body := range map[string]string{
		"no source": `{"game_serial":"new-game"}`,
		"no target": `{"from_game_serial":"old-game","game_serial":"  "}`,
	} {
		fake := newFakeGameRoom()
		response := httptest.NewRecorder()
		gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodPut,
			"/api/v1/game-rooms/abcdefgh/game", strings.NewReader(body)))

		if response.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, want 422", name, response.Code)
		}
		if fake.rebindCalls != 0 {
			t.Errorf("%s: the service was called %d times, want 0", name, fake.rebindCalls)
		}
	}
}

// A stale source serial and a game on another post both mean "this room is not yours to
// move", and each keeps the status code it already has elsewhere in the room API: the
// mismatch is 403 because the caller failed to prove which game it is on, the cross-post
// move is 409 because the room and the game simply cannot be paired.
func TestRebindGameRoomReportsARefusal(t *testing.T) {
	for name, testCase := range map[string]struct {
		err  error
		want int
	}{
		"stale source": {gameroom.ErrRoomMismatch, http.StatusForbidden},
		"another post": {gameroom.ErrRoomNotRebindable, http.StatusConflict},
		"unknown room": {gameroom.ErrNotFound, http.StatusNotFound},
		"unknown game": {gameroom.ErrGameNotFound, http.StatusNotFound},
	} {
		fake := newFakeGameRoom()
		fake.rebindErr = testCase.err
		response := httptest.NewRecorder()
		gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodPut,
			"/api/v1/game-rooms/abcdefgh/game",
			strings.NewReader(`{"from_game_serial":"old-game","game_serial":"new-game"}`)))

		if response.Code != testCase.want {
			t.Errorf("%s: status = %d, want %d; body = %s",
				name, response.Code, testCase.want, response.Body.String())
		}
	}
}

// One call returns everything a joining client needs. Laravel needed three, each able to
// see a different moment and each racing to create the participant row.
func TestGameRoomStateReturnsPlayerVotesBetAndBoardTogether(t *testing.T) {
	fake := newFakeGameRoom()
	fake.votesFound = true
	fake.votes = gameroom.VoteTally{
		FirstCandidate: 11, SecondCandidate: 22,
		FirstCandidateVotes: 3, SecondCandidateVote: 1, RemainElements: 8, TotalVotes: 4,
	}
	fake.betFound = true
	fake.latestBet = gameroom.PlacedBet{WinnerID: 11, LoserID: 22, CurrentRound: 2, OfRound: 4, RemainElements: 8}

	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/game-rooms/abcdefgh?anonymous_id=browser-a", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}

	var body struct {
		Serial     string `json:"serial"`
		GameSerial string `json:"game_serial"`
		Player     *struct {
			PlayerID string `json:"player_id"`
			Name     string `json:"name"`
			Accuracy string `json:"accuracy"`
		} `json:"player"`
		Votes *struct {
			TotalVotes int `json:"total_votes"`
		} `json:"votes"`
		LatestBet *struct {
			WinnerID int64 `json:"winner_id"`
		} `json:"latest_bet"`
		Leaderboard *json.RawMessage `json:"leaderboard"`
	}
	decodeData(t, response, &body)

	if body.Serial != "abcdefgh" || body.GameSerial != "game-serial" {
		t.Errorf("serials = %q / %q", body.Serial, body.GameSerial)
	}
	if body.Player == nil {
		t.Fatal("no player in the response")
	}
	// The digest, never the row id: a room link is public.
	if body.Player.PlayerID == "" || body.Player.PlayerID == "7" {
		t.Errorf("player_id = %q, want the digest rather than the row id", body.Player.PlayerID)
	}
	// A string, because the client interpolates it into "勝率:{}%".
	if body.Player.Accuracy != "63.49" {
		t.Errorf("accuracy = %q, want \"63.49\"", body.Player.Accuracy)
	}
	if body.Votes == nil || body.Votes.TotalVotes != 4 {
		t.Errorf("votes = %+v", body.Votes)
	}
	if body.LatestBet == nil || body.LatestBet.WinnerID != 11 {
		t.Errorf("latest bet = %+v", body.LatestBet)
	}
	if body.Leaderboard == nil {
		t.Error("no leaderboard in the response")
	}
	if fake.joinCalls != 1 {
		t.Errorf("Join was called %d times, want 1", fake.joinCalls)
	}
}

// The starting nickname is drawn in the visitor's language, so the join has to carry one.
// The SPA sets Accept-Language from the localized route it is on.
func TestGameRoomStateForwardsTheLocaleToJoin(t *testing.T) {
	for name, test := range map[string]string{
		"header": "zh-TW,zh;q=0.9",
		"none":   "",
	} {
		t.Run(name, func(t *testing.T) {
			fake := newFakeGameRoom()
			request := httptest.NewRequest(http.MethodGet,
				"/api/v1/game-rooms/abcdefgh?anonymous_id=browser-a", nil)
			if test != "" {
				request.Header.Set("Accept-Language", test)
			}
			gameRoomHandler(fake).ServeHTTP(httptest.NewRecorder(), request)

			if fake.lastLocale != test {
				t.Errorf("locale = %q, want %q", fake.lastLocale, test)
			}
		})
	}
}

// Votes and the latest bet are absent rather than zeroed when there is nothing to report:
// a client must be able to tell "no pairing yet" from "nobody has voted".
func TestGameRoomStateOmitsAbsentVotesAndBets(t *testing.T) {
	fake := newFakeGameRoom()
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/game-rooms/abcdefgh?anonymous_id=browser-a", nil))

	var body struct {
		Votes     *json.RawMessage `json:"votes"`
		LatestBet *json.RawMessage `json:"latest_bet"`
	}
	decodeData(t, response, &body)
	if body.Votes != nil {
		t.Errorf("votes = %s, want null", *body.Votes)
	}
	if body.LatestBet != nil {
		t.Errorf("latest_bet = %s, want null", *body.LatestBet)
	}
}

// THE SHARED-IDENTITY REFUSAL. Laravel defaulted a missing anonymous id to "unknown",
// which put every visitor without a session on one participant row.
func TestGameRoomEndpointsRefuseAMissingAnonymousID(t *testing.T) {
	cases := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"state", http.MethodGet, "/api/v1/game-rooms/abcdefgh", ""},
		{"state with blanks", http.MethodGet, "/api/v1/game-rooms/abcdefgh?anonymous_id=%20%20", ""},
		{"bet", http.MethodPost, "/api/v1/game-rooms/abcdefgh/bets", `{"winner_id":1,"loser_id":2}`},
		{"rename", http.MethodPut, "/api/v1/game-rooms/abcdefgh/player", `{"nickname":"新名字"}`},
	}

	for _, testCase := range cases {
		fake := newFakeGameRoom()
		var reader io.Reader
		if testCase.body != "" {
			reader = strings.NewReader(testCase.body)
		}
		response := httptest.NewRecorder()
		gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(testCase.method, testCase.target, reader))

		if response.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, want 422", testCase.name, response.Code)
		}
		if fake.joinCalls != 0 {
			t.Errorf("%s: the service was reached without an anonymous id", testCase.name)
		}
	}
}

func TestGameRoomStateReportsAnUnknownRoom(t *testing.T) {
	fake := newFakeGameRoom()
	fake.roomFound = false
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/game-rooms/nosuchroom?anonymous_id=browser-a", nil))

	if response.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", response.Code)
	}
}

// A stale link naming a different game is refused rather than silently joining the wrong
// room, which is what Laravel's getRoomVotes answered 403 for.
func TestGameRoomStateRejectsAMismatchedGameSerial(t *testing.T) {
	fake := newFakeGameRoom()
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/game-rooms/abcdefgh?anonymous_id=browser-a&game_serial=some-other-game", nil))

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
	if fake.joinCalls != 0 {
		t.Error("a mismatched link still created a participant")
	}
}

func TestPlacingABetAnswers204(t *testing.T) {
	fake := newFakeGameRoom()
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodPost,
		"/api/v1/game-rooms/abcdefgh/bets", strings.NewReader(
			`{"anonymous_id":"browser-a","winner_id":11,"loser_id":22}`)))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body = %s", response.Code, response.Body.String())
	}
	if fake.betCalls != 1 {
		t.Fatalf("Bet was called %d times", fake.betCalls)
	}
	if fake.lastBet.WinnerID != 11 || fake.lastBet.LoserID != 22 {
		t.Errorf("the pick was not passed through: %+v", fake.lastBet)
	}
	// The game serial reaches the service, which is how it finds the round in progress.
	if fake.lastBetGameSerial != "game-serial" {
		t.Errorf("game serial = %q", fake.lastBetGameSerial)
	}
}

// Between matches the request is well formed but there is nothing to wager on. 409 says
// "retry once the host advances", which 422 would not.
func TestBettingBetweenRoundsIs409(t *testing.T) {
	for failure, wantCode := range map[error]string{
		gameroom.ErrNoRoundInProgress:    "no_round_in_progress",
		gameroom.ErrNotTheCurrentPairing: "stale_pairing",
	} {
		fake := newFakeGameRoom()
		fake.betErr = failure
		response := httptest.NewRecorder()
		gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodPost,
			"/api/v1/game-rooms/abcdefgh/bets",
			strings.NewReader(`{"anonymous_id":"browser-a","winner_id":11,"loser_id":22}`)))

		if response.Code != http.StatusConflict {
			t.Errorf("%v: status = %d, want 409", failure, response.Code)
		}
		if !strings.Contains(response.Body.String(), wantCode) {
			t.Errorf("%v: body does not carry %q: %s", failure, wantCode, response.Body.String())
		}
	}
}

func TestARefusedBetIs500NotSilent(t *testing.T) {
	fake := newFakeGameRoom()
	fake.betErr = errors.New("the round is not valid")
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodPost,
		"/api/v1/game-rooms/abcdefgh/bets", strings.NewReader(
			`{"anonymous_id":"browser-a","winner_id":11,"loser_id":11}`)))

	if response.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", response.Code)
	}
	if strings.Contains(response.Body.String(), "the round is not valid") {
		t.Errorf("the internal message leaked: %s", response.Body.String())
	}
}

func TestRenamingAnswersWithTheNewName(t *testing.T) {
	fake := newFakeGameRoom()
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodPut,
		"/api/v1/game-rooms/abcdefgh/player",
		strings.NewReader(`{"anonymous_id":"browser-a","nickname":"  新名字  "}`)))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Name string `json:"name"`
	}
	decodeData(t, response, &body)
	// Trimmed, so the client shows what was stored rather than what was typed.
	if body.Name != "新名字" {
		t.Errorf("name = %q", body.Name)
	}
	if fake.lastNickname != "  新名字  " {
		t.Errorf("the handler pre-trimmed the name: %q; trimming belongs to the service", fake.lastNickname)
	}
}

func TestRenamingTooSoonIs429(t *testing.T) {
	fake := newFakeGameRoom()
	fake.renameErr = gameroom.ErrNicknameTooSoon
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodPut,
		"/api/v1/game-rooms/abcdefgh/player",
		strings.NewReader(`{"anonymous_id":"browser-a","nickname":"新名字"}`)))

	if response.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", response.Code)
	}
}

func TestAnInvalidNicknameIs422(t *testing.T) {
	fake := newFakeGameRoom()
	fake.renameErr = gameroom.ErrInvalidNickname
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodPut,
		"/api/v1/game-rooms/abcdefgh/player",
		strings.NewReader(`{"anonymous_id":"browser-a","nickname":"太長太長太長太長太長太長"}`)))

	if response.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", response.Code)
	}
}

// A room is playable without an account, and identity inside it is the browser id. The
// account is passed through when present, for the audit column only.
func TestTheAccountIsOptionalAndPassedThroughWhenPresent(t *testing.T) {
	fake := newFakeGameRoom()
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/game-rooms/abcdefgh?anonymous_id=browser-a", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("an anonymous request was refused: %d", response.Code)
	}
	if fake.lastUserID != nil {
		t.Errorf("user id = %v on an anonymous request", *fake.lastUserID)
	}

	fake = newFakeGameRoom()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/game-rooms/abcdefgh?anonymous_id=browser-a", nil)
	request.Header.Set("Authorization", "Bearer a.valid.token")
	gameRoomHandler(fake).ServeHTTP(httptest.NewRecorder(), request)
	if fake.lastUserID == nil || *fake.lastUserID != 42 {
		t.Errorf("user id = %v, want 42 from the token", fake.lastUserID)
	}
	// The browser id still decides identity, not the account.
	if fake.lastAnonymousID != "browser-a" {
		t.Errorf("anonymous id = %q", fake.lastAnonymousID)
	}
}

func TestTheLeaderboardPollEndpointAnswersTheBoard(t *testing.T) {
	fake := newFakeGameRoom()
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/game-rooms/abcdefgh/leaderboard", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	// No anonymous id required: this one reads the room without joining it.
	if fake.joinCalls != 0 {
		t.Error("the poll endpoint created a participant")
	}
}

func TestTheVotesEndpointAnswersTheTallyWithoutJoining(t *testing.T) {
	fake := newFakeGameRoom()
	fake.votes = gameroom.VoteTally{
		FirstCandidate: 11, SecondCandidate: 12,
		FirstCandidateVotes: 4, SecondCandidateVote: 2,
		RemainElements: 8, TotalVotes: 6, CurrentRound: 3, OfRound: 4,
	}
	fake.votesFound = true
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/game-rooms/abcdefgh/votes?game_serial=game-serial", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Votes *gameroom.VoteTally `json:"votes"`
	}
	decodeData(t, response, &body)
	if body.Votes == nil {
		t.Fatal("votes = null, want the tally")
	}
	if body.Votes.TotalVotes != fake.votes.TotalVotes {
		t.Errorf("total_votes = %d, want %d", body.Votes.TotalVotes, fake.votes.TotalVotes)
	}
	// The host reads their own room with this. Joining would put them on its leaderboard.
	if fake.joinCalls != 0 {
		t.Error("the votes endpoint created a participant")
	}
}

func TestTheVotesEndpointAnswersNullBetweenRounds(t *testing.T) {
	fake := newFakeGameRoom()
	fake.votesFound = false
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/game-rooms/abcdefgh/votes", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Votes *gameroom.VoteTally `json:"votes"`
	}
	decodeData(t, response, &body)
	if body.Votes != nil {
		t.Errorf("votes = %+v, want null", body.Votes)
	}
}

func TestTheVotesEndpointRejectsAMismatchedGameSerial(t *testing.T) {
	fake := newFakeGameRoom()
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/game-rooms/abcdefgh/votes?game_serial=another-game", nil))

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", response.Code, response.Body.String())
	}
}

func TestGameRoomEndpointsAnswer503WhenUnconfigured(t *testing.T) {
	handler := New(Options{
		Environment: "test",
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	for _, target := range []string{
		"/api/v1/game-rooms/abcdefgh?anonymous_id=browser-a",
		"/api/v1/game-rooms/abcdefgh/leaderboard",
		"/api/v1/game-rooms/abcdefgh/votes",
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: status = %d, want 503", target, response.Code)
		}
	}
}

func TestGameRoomStateIsNotCacheable(t *testing.T) {
	fake := newFakeGameRoom()
	response := httptest.NewRecorder()
	gameRoomHandler(fake).ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/game-rooms/abcdefgh?anonymous_id=browser-a", nil))

	// The response names one player's score. A shared cache entry would show it to the
	// next visitor.
	if control := response.Header().Get("Cache-Control"); !strings.Contains(control, "private") &&
		!strings.Contains(control, "no-store") {
		t.Errorf("Cache-Control = %q, want it private", control)
	}
}
