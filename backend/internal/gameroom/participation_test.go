package gameroom

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeParticipation records what the service asked of the store.
type fakeParticipation struct {
	room        Room
	roomCreated bool
	gameSerial  string
	participant Participant

	previousCombo int
	previousWon   bool
	previousFound bool
	round         RoundInProgress
	hosting       bool
	onScreenPair  [2]int64

	lastBetCombo   int
	lastBetRoomID  int64
	lastBetPlayer  int64
	lastBet        PlacedBet
	betCalls       int
	lastRenamedTo  string
	renameCalls    int
	ensureRoomCall int
	lastNewSerial  string
	rebindCalls    int
	rebindFrom     string
	rebindTo       string
	rebindErr      error

	err error
}

func (fake *fakeParticipation) EnsureRoom(_ context.Context, gameSerial, newSerial string) (Room, bool, error) {
	fake.ensureRoomCall++
	fake.gameSerial, fake.lastNewSerial = gameSerial, newSerial
	if fake.err != nil {
		return Room{}, false, fake.err
	}
	return fake.room, fake.roomCreated, nil
}

func (fake *fakeParticipation) RebindRoom(
	_ context.Context, _, fromGameSerial, toGameSerial string,
) (Room, error) {
	fake.rebindCalls++
	fake.rebindFrom, fake.rebindTo = fromGameSerial, toGameSerial
	if fake.rebindErr != nil {
		return Room{}, fake.rebindErr
	}
	if fake.err != nil {
		return Room{}, fake.err
	}
	return fake.room, nil
}

func (fake *fakeParticipation) RoomBySerialWithGame(_ context.Context, _ string) (Room, string, bool, error) {
	return fake.room, fake.gameSerial, true, fake.err
}

func (fake *fakeParticipation) EnsureParticipant(
	_ context.Context, _ int64, anonymousID string, userID *int64, nickname string, startingScore int,
) (Participant, error) {
	if fake.err != nil {
		return Participant{}, fake.err
	}
	participant := fake.participant
	participant.AnonymousID = anonymousID
	participant.UserID = userID
	if participant.Nickname == "" {
		participant.Nickname = nickname
	}
	if participant.Score == 0 {
		participant.Score = startingScore
	}
	return participant, nil
}

func (fake *fakeParticipation) UpsertBet(
	_ context.Context, roomID, participantID int64, bet PlacedBet, lastCombo int,
) error {
	fake.betCalls++
	fake.lastBetRoomID, fake.lastBetPlayer, fake.lastBet, fake.lastBetCombo = roomID, participantID, bet, lastCombo
	return fake.err
}

func (fake *fakeParticipation) PreviousBetStreak(_ context.Context, _ int64) (int, bool, bool, error) {
	if fake.err != nil {
		return 0, false, false, fake.err
	}
	return fake.previousCombo, fake.previousWon, fake.previousFound, nil
}

func (fake *fakeParticipation) SetOnScreenPair(_ context.Context, _ string, first, second int64) error {
	fake.onScreenPair = [2]int64{first, second}
	return fake.err
}

func (fake *fakeParticipation) RoomByGameSerial(_ context.Context, _ string) (Room, bool, error) {
	if fake.err != nil {
		return Room{}, false, fake.err
	}
	return fake.room, fake.hosting, nil
}

func (fake *fakeParticipation) RoundInProgress(_ context.Context, _ string) (RoundInProgress, error) {
	if fake.err != nil {
		return RoundInProgress{}, fake.err
	}
	return fake.round, nil
}

func (fake *fakeParticipation) Rename(_ context.Context, _ int64, nickname string) error {
	fake.renameCalls++
	fake.lastRenamedTo = nickname
	return fake.err
}

func (fake *fakeParticipation) CurrentVotes(_ context.Context, _ int64, _ string) (VoteTally, bool, error) {
	return VoteTally{}, false, fake.err
}

func (fake *fakeParticipation) LatestBet(_ context.Context, _ int64) (PlacedBet, bool, error) {
	return PlacedBet{}, false, fake.err
}

func newParticipationService(t *testing.T, store *fakeParticipation, limiter RenameLimiter) *Participation {
	t.Helper()
	service, err := NewParticipation(ParticipationOptions{
		Repository: store,
		Limiter:    limiter,
		Scoring:    DefaultScoring(),
		Nicknames:  func(string) string { return "測試路人" },
	})
	if err != nil {
		t.Fatalf("NewParticipation() error = %v", err)
	}
	return service
}

// THE STREAK RULE. last_combo is multiplied by ComboScore when the wager settles, so
// getting it wrong is a scoring error rather than a display one. A win continues the
// streak; a loss, an unsettled previous wager, or no previous wager at all resets it.
func TestBetCarriesTheStreakOnlyAfterAWin(t *testing.T) {
	cases := []struct {
		name          string
		previousCombo int
		previousWon   bool
		previousFound bool
		wantCombo     int
	}{
		{"first wager of the game", 0, false, false, 0},
		{"after a win from zero", 0, true, true, 1},
		{"after a win on a streak of three", 3, true, true, 4},
		{"after a loss", 5, false, true, 0},
		// A previous wager that has not settled yet reads as not-won, so the streak
		// resets. That matches Laravel, which tested won_at !== null.
		{"after an unsettled wager", 5, false, true, 0},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			store := &fakeParticipation{
				previousCombo: testCase.previousCombo,
				previousWon:   testCase.previousWon,
				previousFound: testCase.previousFound,
				participant:   Participant{ID: 7},
			}
			service := newParticipationService(t, store, nil)

			err := service.Bet(context.Background(), 42, Participant{ID: 7},
				PlacedBet{WinnerID: 1, LoserID: 2, CurrentRound: 3, OfRound: 4, RemainElements: 5})
			if err != nil {
				t.Fatalf("Bet() error = %v", err)
			}
			if store.lastBetCombo != testCase.wantCombo {
				t.Errorf("last_combo = %d, want %d", store.lastBetCombo, testCase.wantCombo)
			}
			if store.lastBetRoomID != 42 || store.lastBetPlayer != 7 {
				t.Errorf("wager recorded against room %d player %d", store.lastBetRoomID, store.lastBetPlayer)
			}
		})
	}
}

func TestBetRejectsAnImpossibleWager(t *testing.T) {
	cases := map[string]PlacedBet{
		"no winner":          {WinnerID: 0, LoserID: 2, CurrentRound: 1, OfRound: 2, RemainElements: 3},
		"no loser":           {WinnerID: 1, LoserID: 0, CurrentRound: 1, OfRound: 2, RemainElements: 3},
		"voting for itself":  {WinnerID: 1, LoserID: 1, CurrentRound: 1, OfRound: 2, RemainElements: 3},
		"round zero":         {WinnerID: 1, LoserID: 2, CurrentRound: 0, OfRound: 2, RemainElements: 3},
		"of round zero":      {WinnerID: 1, LoserID: 2, CurrentRound: 1, OfRound: 0, RemainElements: 3},
		"negative remainder": {WinnerID: 1, LoserID: 2, CurrentRound: 1, OfRound: 2, RemainElements: -1},
	}
	for name, bet := range cases {
		store := &fakeParticipation{participant: Participant{ID: 7}}
		service := newParticipationService(t, store, nil)
		if err := service.Bet(context.Background(), 42, Participant{ID: 7}, bet); err == nil {
			t.Errorf("%s: Bet() succeeded", name)
		}
		if store.betCalls != 0 {
			t.Errorf("%s: an invalid wager reached the store", name)
		}
	}
}

// A missing anonymous id is refused rather than defaulted. Laravel used the literal
// "unknown", which put every visitor with no session on ONE participant row: shared score,
// shared wagers.
func TestJoinRefusesAnEmptyAnonymousID(t *testing.T) {
	store := &fakeParticipation{participant: Participant{ID: 7}}
	service := newParticipationService(t, store, nil)

	for _, value := range []string{"", "   ", "\t"} {
		if _, err := service.Join(context.Background(), 42, value, nil, "zh-tw"); err == nil {
			t.Errorf("Join(%q) succeeded", value)
		}
	}
}

func TestJoinSuppliesAStartingNameAndScore(t *testing.T) {
	store := &fakeParticipation{}
	service := newParticipationService(t, store, nil)

	participant, err := service.Join(context.Background(), 42, "browser-a", nil, "zh-tw")
	if err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	if participant.Nickname != "測試路人" {
		t.Errorf("nickname = %q, want the generated one", participant.Nickname)
	}
	if participant.Score != DefaultScoring().DefaultScore {
		t.Errorf("score = %d, want %d", participant.Score, DefaultScoring().DefaultScore)
	}
}

func TestRenameValidatesTheName(t *testing.T) {
	cases := map[string]string{
		"empty":          "",
		"whitespace":     "    ",
		"over ten runes": strings.Repeat("排", MaxNicknameRunes+1),
	}
	for name, nickname := range cases {
		store := &fakeParticipation{}
		service := newParticipationService(t, store, NewMemoryRenameLimiter())
		err := service.Rename(context.Background(), Participant{ID: 7}, nickname)
		if !errors.Is(err, ErrInvalidNickname) {
			t.Errorf("%s: error = %v, want ErrInvalidNickname", name, err)
		}
		if store.renameCalls != 0 {
			t.Errorf("%s: an invalid name reached the store", name)
		}
	}

	// Exactly the limit is accepted, counted in runes: ten Chinese characters are thirty
	// bytes, and a byte limit would reject the longest name the old form allowed.
	store := &fakeParticipation{}
	service := newParticipationService(t, store, NewMemoryRenameLimiter())
	if err := service.Rename(context.Background(), Participant{ID: 7},
		strings.Repeat("排", MaxNicknameRunes)); err != nil {
		t.Errorf("a name at the exact limit was refused: %v", err)
	}
}

func TestRenameTrimsTheName(t *testing.T) {
	store := &fakeParticipation{}
	service := newParticipationService(t, store, NewMemoryRenameLimiter())

	if err := service.Rename(context.Background(), Participant{ID: 7}, "  新名字  "); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if store.lastRenamedTo != "新名字" {
		t.Errorf("stored %q, want it trimmed", store.lastRenamedTo)
	}
}

func TestRenameIsRateLimited(t *testing.T) {
	store := &fakeParticipation{}
	limiter := NewMemoryRenameLimiter()
	now := time.Now()
	limiter.now = func() time.Time { return now }
	service := newParticipationService(t, store, limiter)

	if err := service.Rename(context.Background(), Participant{ID: 7}, "第一"); err != nil {
		t.Fatalf("first rename: %v", err)
	}
	if err := service.Rename(context.Background(), Participant{ID: 7}, "第二"); !errors.Is(err, ErrNicknameTooSoon) {
		t.Fatalf("second rename error = %v, want ErrNicknameTooSoon", err)
	}
	if store.renameCalls != 1 {
		t.Errorf("%d renames reached the store, want 1", store.renameCalls)
	}

	// Another player is not blocked by the first one's cooldown.
	if err := service.Rename(context.Background(), Participant{ID: 8}, "別人"); err != nil {
		t.Errorf("a different player was blocked: %v", err)
	}

	// And the first player may rename again once the window passes.
	now = now.Add(NicknameCooldown + time.Second)
	if err := service.Rename(context.Background(), Participant{ID: 7}, "第三"); err != nil {
		t.Errorf("the cooldown did not expire: %v", err)
	}
}

// Without a limiter the rename still works. A single-instance deployment with no Redis
// must not lose the feature entirely.
func TestRenameWorksWithoutALimiter(t *testing.T) {
	store := &fakeParticipation{}
	service := newParticipationService(t, store, nil)

	for range 3 {
		if err := service.Rename(context.Background(), Participant{ID: 7}, "名字"); err != nil {
			t.Fatalf("Rename() error = %v", err)
		}
	}
	if store.renameCalls != 3 {
		t.Errorf("%d renames reached the store, want 3", store.renameCalls)
	}
}

func TestEnsureRoomGeneratesASerialAndRejectsAnEmptyGame(t *testing.T) {
	store := &fakeParticipation{room: Room{ID: 5, Serial: "abcdefgh"}, roomCreated: true}
	service := newParticipationService(t, store, nil)

	room, created, err := service.EnsureRoom(context.Background(), "game-serial", nil)
	if err != nil {
		t.Fatalf("EnsureRoom() error = %v", err)
	}
	if !created || room.ID != 5 {
		t.Errorf("created = %v, room = %+v", created, room)
	}
	if len(store.lastNewSerial) != RoomSerialLength {
		t.Errorf("the generated serial %q is not %d characters", store.lastNewSerial, RoomSerialLength)
	}

	if _, _, err := service.EnsureRoom(context.Background(), "   ", nil); !errors.Is(err, ErrGameNotFound) {
		t.Errorf("error = %v, want ErrGameNotFound for an empty game serial", err)
	}
}

// A host opens the room mid-game, so the pair already on screen has to be recorded — or
// the first participants are shown the match that was just decided.
func TestEnsureRoomRecordsThePairAlreadyOnScreen(t *testing.T) {
	store := &fakeParticipation{room: Room{ID: 5, Serial: "abcdefgh"}, roomCreated: true}
	service := newParticipationService(t, store, nil)

	if _, _, err := service.EnsureRoom(context.Background(), "game-serial", []int64{11, 22}); err != nil {
		t.Fatalf("EnsureRoom() error = %v", err)
	}
	if store.onScreenPair != [2]int64{11, 22} {
		t.Errorf("recorded pair = %v, want [11 22]", store.onScreenPair)
	}

	// Nothing to record when the host does not say, which is the solo case.
	store.onScreenPair = [2]int64{}
	if _, _, err := service.EnsureRoom(context.Background(), "game-serial", nil); err != nil {
		t.Fatalf("EnsureRoom() error = %v", err)
	}
	if store.onScreenPair != [2]int64{} {
		t.Errorf("a pair was recorded with none supplied: %v", store.onScreenPair)
	}
}

/**
 * The restart case: the room follows the host into the new game, and the pair the host has
 * up in it is recorded — otherwise the participants are shown a game whose candidates
 * column is still empty.
 */
func TestRebindMovesTheRoomAndRecordsTheNewPair(t *testing.T) {
	store := &fakeParticipation{room: Room{ID: 7, Serial: "ROOMSERIAL"}}
	service := newParticipationService(t, store, nil)

	room, err := service.Rebind(context.Background(), "ROOMSERIAL", "old-game", "new-game", []int64{11, 22})
	if err != nil {
		t.Fatalf("Rebind() error = %v", err)
	}
	if room.Serial != "ROOMSERIAL" {
		t.Errorf("room serial = %q, want the room to keep its serial", room.Serial)
	}
	if store.rebindFrom != "old-game" || store.rebindTo != "new-game" {
		t.Errorf("rebind %q -> %q, want old-game -> new-game", store.rebindFrom, store.rebindTo)
	}
	if store.onScreenPair != [2]int64{11, 22} {
		t.Errorf("on screen pair = %v, want the new game's pair", store.onScreenPair)
	}
}

func TestRebindNeedsBothGameSerials(t *testing.T) {
	store := &fakeParticipation{room: Room{ID: 7, Serial: "ROOMSERIAL"}}
	service := newParticipationService(t, store, nil)

	for name, call := range map[string]func() error{
		"no room": func() error {
			_, err := service.Rebind(context.Background(), " ", "old-game", "new-game", nil)
			return err
		},
		"no source game": func() error {
			_, err := service.Rebind(context.Background(), "ROOMSERIAL", "", "new-game", nil)
			return err
		},
		"no target game": func() error {
			_, err := service.Rebind(context.Background(), "ROOMSERIAL", "old-game", " ", nil)
			return err
		},
	} {
		if err := call(); err == nil {
			t.Errorf("Rebind() with %s should fail", name)
		}
	}
	if store.rebindCalls != 0 {
		t.Errorf("rebind calls = %d, want the store left alone", store.rebindCalls)
	}
}

// A refusal from the store is the caller's answer, not something to paper over: it means
// the room has already moved, or the game belongs to another post.
func TestRebindSurfacesARefusal(t *testing.T) {
	store := &fakeParticipation{room: Room{ID: 7, Serial: "ROOMSERIAL"}, rebindErr: ErrRoomNotRebindable}
	service := newParticipationService(t, store, nil)

	if _, err := service.Rebind(context.Background(), "ROOMSERIAL", "old-game", "new-game", []int64{1, 2}); !errors.Is(err, ErrRoomNotRebindable) {
		t.Fatalf("Rebind() error = %v, want ErrRoomNotRebindable", err)
	}
	if store.onScreenPair != [2]int64{} {
		t.Errorf("on screen pair = %v, want nothing recorded for a refused move", store.onScreenPair)
	}
}

func TestRandomNicknameStaysWithinTheColumn(t *testing.T) {
	// English is the case that needs the guard: "Adventurous" and "Hippopotamus" are both
	// in the site's own lists and together they overflow the column.
	for _, locale := range []string{"zh-tw", "en-US", "ja", "de", "", "en;q=0.9,zh-TW"} {
		for range 200 {
			nickname := RandomNickname(locale)
			if nickname == "" {
				t.Fatalf("an empty nickname was generated for %q", locale)
			}
			if runes := len([]rune(nickname)); runes > NicknameColumnRunes {
				t.Fatalf("%q (%s) is %d runes, over the %d the column allows",
					nickname, locale, runes, NicknameColumnRunes)
			}
		}
	}
}

func TestRandomNicknameFollowsTheLocale(t *testing.T) {
	// The nickname is the first thing a player sees of themselves in a room, so it is
	// drawn from the language they are reading the page in. An unknown language falls
	// back to the site's own rather than to no name at all.
	cases := map[string]string{
		"zh-tw":          "zh_TW",
		"zh-TW,zh;q=0.9": "zh_TW",
		"en-US":          "en",
		"en;q=0.9,zh-TW": "en",
		"ja":             "ja",
		"de":             defaultNicknameLocale,
		"":               defaultNicknameLocale,
	}
	for locale, want := range cases {
		words := nicknamesByLocale[want]
		for range 50 {
			name := RandomNickname(locale)
			if !fromWordLists(name, words) {
				t.Fatalf("RandomNickname(%q) = %q, which is not in the %s lists", locale, name, want)
			}
		}
	}
}

// fromWordLists reports whether a name is one of the language's adjectives followed by
// one of its animals, which is the whole shape random_nickname() ever produced.
func fromWordLists(name string, words nicknameWords) bool {
	for _, adjective := range words.adjectives {
		rest, found := strings.CutPrefix(name, adjective)
		if !found {
			continue
		}
		for _, animal := range words.names {
			if rest == animal {
				return true
			}
		}
	}
	return false
}

// BetOnCurrentRound sends only the pick; the counters come from the server's own view of
// the round, so a participant cannot record a wager against a round that does not exist.
func TestBetOnCurrentRoundUsesTheServersRoundNumbers(t *testing.T) {
	store := &fakeParticipation{
		participant: Participant{ID: 7},
		round: RoundInProgress{
			FirstCandidate: 11, SecondCandidate: 22, HasPairing: true,
			RemainElements: 34, CurrentRound: 31, OfRound: 32,
		},
	}
	service := newParticipationService(t, store, nil)

	if err := service.BetOnCurrentRound(context.Background(), 42, Participant{ID: 7},
		"game-serial", 22, 11); err != nil {
		t.Fatalf("BetOnCurrentRound() error = %v", err)
	}
	if store.lastBet.CurrentRound != 31 || store.lastBet.OfRound != 32 || store.lastBet.RemainElements != 34 {
		t.Errorf("the wager carried %+v, want the server's 31/32/34", store.lastBet)
	}
	// Either direction of the pairing is a legitimate pick.
	if store.lastBet.WinnerID != 22 || store.lastBet.LoserID != 11 {
		t.Errorf("pick = %d over %d", store.lastBet.WinnerID, store.lastBet.LoserID)
	}
}

func TestBetOnCurrentRoundRefusesWhenNoPairingIsUp(t *testing.T) {
	store := &fakeParticipation{participant: Participant{ID: 7}, round: RoundInProgress{HasPairing: false}}
	service := newParticipationService(t, store, nil)

	err := service.BetOnCurrentRound(context.Background(), 42, Participant{ID: 7}, "game-serial", 11, 22)
	if !errors.Is(err, ErrNoRoundInProgress) {
		t.Fatalf("error = %v, want ErrNoRoundInProgress", err)
	}
	if store.betCalls != 0 {
		t.Error("a wager was recorded with no pairing on screen")
	}
}

// A stale page must not be able to wager on a matchup that is no longer on screen: the
// settlement matches on the pairing, so such a wager would never resolve and would sit in
// the table forever.
func TestBetOnCurrentRoundRefusesAStalePairing(t *testing.T) {
	store := &fakeParticipation{
		participant: Participant{ID: 7},
		round: RoundInProgress{
			FirstCandidate: 11, SecondCandidate: 22, HasPairing: true,
			RemainElements: 34, CurrentRound: 31, OfRound: 32,
		},
	}
	service := newParticipationService(t, store, nil)

	for name, pick := range map[string][2]int64{
		"both elements from an older round": {99, 98},
		"one stale element":                 {11, 98},
		"the other stale element":           {98, 22},
	} {
		store.betCalls = 0
		err := service.BetOnCurrentRound(context.Background(), 42, Participant{ID: 7},
			"game-serial", pick[0], pick[1])
		if !errors.Is(err, ErrNotTheCurrentPairing) {
			t.Errorf("%s: error = %v, want ErrNotTheCurrentPairing", name, err)
		}
		if store.betCalls != 0 {
			t.Errorf("%s: the stale wager was recorded", name)
		}
	}
}

// THE BRACKET RULE, from the observed sequence: (64 of 64, 64 remaining) is followed by
// (1 of 32, 63 remaining). of_round is fixed when a bracket starts, which is why a row can
// read "30 of 32" while 34 elements remain.
func TestNextRoundAdvancesWithinABracketAndStartsTheNextOne(t *testing.T) {
	cases := []struct {
		name                string
		lastCurrent, lastOf int
		remain              int
		wantCurrent, wantOf int
	}{
		{"no match played yet, 128 elements", 0, 0, 128, 1, 64},
		{"no match played yet, odd count", 0, 0, 9, 1, 5},
		{"mid bracket", 30, 32, 34, 31, 32},
		{"one before the end of a bracket", 63, 64, 65, 64, 64},
		// The boundary that matters: the observed row after (64,64,64) is (1,32,63).
		{"last match of a bracket", 64, 64, 64, 1, 32},
		{"last match of a small bracket", 32, 32, 32, 1, 16},
		{"down to the final pair", 1, 1, 2, 1, 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			current, of := NextRound(testCase.lastCurrent, testCase.lastOf, testCase.remain)
			if current != testCase.wantCurrent || of != testCase.wantOf {
				t.Errorf("NextRound(%d, %d, %d) = %d of %d, want %d of %d",
					testCase.lastCurrent, testCase.lastOf, testCase.remain,
					current, of, testCase.wantCurrent, testCase.wantOf)
			}
		})
	}
}

func TestNewParticipationRejectsAMissingRepository(t *testing.T) {
	if _, err := NewParticipation(ParticipationOptions{}); err == nil {
		t.Fatal("NewParticipation() succeeded with no repository")
	}
}
