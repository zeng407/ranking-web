package gameplay

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"2pick.app/backend/internal/postaccess"
)

const maxGameElements = 1024

type MySQLRepository struct {
	database *sql.DB
	now      func() time.Time
	// adult is the deployment's answer to whether an 18+ post needs an account. Held
	// here rather than read per call so every statement of one request agrees.
	adult        postaccess.AdultPolicy
	capabilityMu sync.RWMutex
	// rankVisibility caches the rank_reports row filter this database supports. See
	// resultRankVisibilityClause.
	rankVisibility *string
}

func NewMySQLRepository(database *sql.DB, adult postaccess.AdultPolicy) *MySQLRepository {
	return &MySQLRepository{database: database, now: time.Now, adult: adult}
}

func (repository *MySQLRepository) Definition(
	ctx context.Context, postSerial string, caller postaccess.Caller,
) (Definition, error) {
	visible, visibleArguments := postaccess.VisibilityClause("p", "pp", caller)
	var definition Definition
	err := repository.database.QueryRowContext(ctx, `
		SELECT p.title, p.serial, COALESCE(p.description, ''), p.is_censored, COUNT(pe.id)
		FROM posts p
		JOIN post_policies pp ON pp.post_id = p.id AND `+visible+`
		JOIN post_elements pe ON pe.post_id = p.id
		JOIN elements e ON e.id = pe.element_id AND e.deleted_at IS NULL
		WHERE p.serial = ? AND p.deleted_at IS NULL
		GROUP BY p.id, p.title, p.serial, p.description, p.is_censored
		LIMIT 1`, append(visibleArguments, postSerial)...).Scan(
		&definition.Title, &definition.Serial, &definition.Description,
		&definition.IsCensored, &definition.ElementsCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Definition{}, ErrNotFound
	}
	if err != nil {
		return Definition{}, err
	}
	definition.MaxElements = min(definition.ElementsCount, maxGameElements)
	// The page renders the sign-in gate itself, so it is told the answer rather than
	// deriving it from is_censored — the rule is a deployment setting the browser has
	// no other way to learn.
	definition.RequiresSignIn = repository.adult.GateApplies(definition.IsCensored)
	definition.Element1, definition.Element2 = repository.previewElements(ctx, postSerial)
	return definition, nil
}

// previewElements reads the two preview options from the denormalized
// public_posts payload that the legacy app maintains. The preview is
// presentational, so any problem resolving it degrades to no preview rather
// than failing the whole definition request.
func (repository *MySQLRepository) previewElements(ctx context.Context, postSerial string) (*PreviewElement, *PreviewElement) {
	var rawData []byte
	err := repository.database.QueryRowContext(ctx, `
		SELECT pp.data
		FROM public_posts pp
		JOIN posts p ON p.id = pp.post_id AND p.deleted_at IS NULL
		WHERE p.serial = ?
		LIMIT 1`, postSerial).Scan(&rawData)
	if err != nil || len(rawData) == 0 {
		return nil, nil
	}

	var payload struct {
		Element1 *PreviewElement `json:"element1"`
		Element2 *PreviewElement `json:"element2"`
	}
	if err := json.Unmarshal(rawData, &payload); err != nil {
		return nil, nil
	}

	return usablePreview(payload.Element1), usablePreview(payload.Element2)
}

// usablePreview drops placeholder entries the legacy payload emits when a post
// has fewer than two renderable elements.
func usablePreview(element *PreviewElement) *PreviewElement {
	if element == nil || element.ID == nil {
		return nil
	}
	if element.URL == nil && element.URL2 == nil {
		return nil
	}
	return element
}

func (repository *MySQLRepository) Create(ctx context.Context, input CreateInput) (Session, error) {
	definition, err := repository.Definition(ctx, input.PostSerial, input.Caller)
	if err != nil {
		return Session{}, err
	}
	// Definition itself stays open so the game page can show the blurred preview to a
	// visitor; starting a game on an adult post is where the account is required.
	if err := repository.adult.RequireSignIn(definition.IsCensored, input.Caller); err != nil {
		return Session{}, err
	}
	if input.ElementCount < 2 || input.ElementCount > definition.MaxElements {
		return Session{}, fmt.Errorf("%w: element count must be between 2 and %d", ErrInvalidElementCount, definition.MaxElements)
	}

	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer transaction.Rollback()

	visible, visibleArguments := postaccess.VisibilityClause("p", "pp", input.Caller)
	var postID int64
	if err := transaction.QueryRowContext(ctx, `
		SELECT p.id
		FROM posts p
		JOIN post_policies pp ON pp.post_id = p.id AND `+visible+`
		WHERE p.serial = ? AND p.deleted_at IS NULL
		LIMIT 1`, append(visibleArguments, input.PostSerial)...).Scan(&postID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		return Session{}, err
	}

	elements, err := queryRandomElements(ctx, transaction, postID, input.ElementCount)
	if err != nil {
		return Session{}, err
	}
	if len(elements) != input.ElementCount {
		return Session{}, fmt.Errorf("requested %d elements but only %d are available", input.ElementCount, len(elements))
	}

	serial, err := newUUID()
	if err != nil {
		return Session{}, err
	}
	now := repository.now()
	result, err := transaction.ExecContext(ctx, `
		INSERT INTO games (serial, post_id, element_count, vote_count, created_at, updated_at)
		VALUES (?, ?, ?, 0, ?, ?)`, serial, postID, input.ElementCount, now, now)
	if err != nil {
		return Session{}, err
	}
	gameID, err := result.LastInsertId()
	if err != nil {
		return Session{}, err
	}
	statement, err := transaction.PrepareContext(ctx, `
		INSERT INTO game_elements (game_id, element_id, win_count, is_eliminated, is_ready)
		VALUES (?, ?, 0, 0, 1)`)
	if err != nil {
		return Session{}, err
	}
	defer statement.Close()
	for _, element := range elements {
		if _, err := statement.ExecContext(ctx, gameID, element.ID); err != nil {
			return Session{}, err
		}
	}
	if err := transaction.Commit(); err != nil {
		return Session{}, err
	}
	return Session{
		GameSerial: serial, ServerVoteCount: 0, Definition: definition, Elements: elements,
	}, nil
}

func (repository *MySQLRepository) Resume(
	ctx context.Context, gameSerial string, caller postaccess.Caller,
) (Session, error) {
	visible, visibleArguments := postaccess.VisibilityClause("p", "pp", caller)
	var gameID int64
	var selectedCount int
	var session Session
	err := repository.database.QueryRowContext(ctx, `
		SELECT g.id, g.serial,
		       (SELECT COUNT(*) FROM game_1v1_rounds gr WHERE gr.game_id = g.id),
		       p.title, p.serial, COALESCE(p.description, ''), p.is_censored,
		       (SELECT COUNT(*) FROM post_elements pe
		        JOIN elements post_element ON post_element.id = pe.element_id AND post_element.deleted_at IS NULL
		        WHERE pe.post_id = p.id),
		       LEAST((SELECT COUNT(*) FROM post_elements pe
		        JOIN elements post_element ON post_element.id = pe.element_id AND post_element.deleted_at IS NULL
		        WHERE pe.post_id = p.id), ?),
		       g.element_count
		FROM games g
		JOIN posts p ON p.id = g.post_id AND p.deleted_at IS NULL
		JOIN post_policies pp ON pp.post_id = p.id AND `+visible+`
		WHERE g.serial = ?
		LIMIT 1`, append(append([]any{maxGameElements}, visibleArguments...), gameSerial)...).Scan(
		&gameID, &session.GameSerial, &session.ServerVoteCount,
		&session.Definition.Title, &session.Definition.Serial, &session.Definition.Description,
		&session.Definition.IsCensored, &session.Definition.ElementsCount, &session.Definition.MaxElements,
		&selectedCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	if err := repository.adult.RequireSignIn(session.Definition.IsCensored, caller); err != nil {
		return Session{}, err
	}
	elements, err := queryGameElements(ctx, repository.database, gameID)
	if err != nil {
		return Session{}, err
	}
	if len(elements) != selectedCount {
		return Session{}, fmt.Errorf("game %s has %d of %d expected elements", gameSerial, len(elements), selectedCount)
	}
	session.Elements = elements
	return session, nil
}

func (repository *MySQLRepository) Result(
	ctx context.Context, gameSerial string, caller postaccess.Caller,
) (GameResult, error) {
	visible, visibleArguments := postaccess.VisibilityClause("p", "pp", caller)
	var gameID int64
	var result GameResult
	var completedAt sql.NullTime
	var isCensored bool
	err := repository.database.QueryRowContext(ctx, `
		SELECT g.id, g.serial, p.serial, g.completed_at, p.is_censored
		FROM games g
		JOIN posts p ON p.id = g.post_id AND p.deleted_at IS NULL
		JOIN post_policies pp ON pp.post_id = p.id AND `+visible+`
		WHERE g.serial = ?
		LIMIT 1`, append(visibleArguments, gameSerial)...).Scan(
		&gameID, &result.GameSerial, &result.PostSerial, &completedAt, &isCensored)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !completedAt.Valid) {
		return GameResult{}, ErrNotFound
	}
	if err != nil {
		return GameResult{}, err
	}
	if err := repository.adult.RequireSignIn(isCensored, caller); err != nil {
		return GameResult{}, err
	}

	rounds, err := queryPersistedRounds(ctx, repository.database, gameID)
	if err != nil {
		return GameResult{}, err
	}
	elementIDs := topResultElementIDs(rounds, 10)
	if len(elementIDs) == 0 {
		return GameResult{}, ErrNotFound
	}

	hiddenClause, err := repository.resultRankVisibilityClause(ctx)
	if err != nil {
		return GameResult{}, err
	}
	itemsByID, err := queryResultElements(ctx, repository.database, gameID, elementIDs, hiddenClause)
	if err != nil {
		return GameResult{}, err
	}
	result.Items = make([]GameResultItem, 0, len(elementIDs))
	for index, elementID := range elementIDs {
		item, exists := itemsByID[elementID]
		if !exists {
			return GameResult{}, fmt.Errorf("game result element %d is unavailable", elementID)
		}
		item.Rank = index + 1
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func queryResultElements(ctx context.Context, queryer queryer, gameID int64, elementIDs []int64, hiddenClause string) (map[int64]GameResultItem, error) {
	placeholders := strings.TrimRight(strings.Repeat("?,", len(elementIDs)), ",")
	arguments := make([]any, 0, len(elementIDs)+1)
	arguments = append(arguments, gameID)
	for _, elementID := range elementIDs {
		arguments = append(arguments, elementID)
	}
	rows, err := queryer.QueryContext(ctx, `
		SELECT e.id, e.source_url, e.thumb_url, e.mediumthumb_url, e.lowthumb_url,
		       e.title, e.type, e.video_start_second, e.video_end_second,
		       e.video_source, e.video_id, e.video_duration_second, ge.win_count,
		       (SELECT rr.rank FROM rank_reports rr
		        WHERE rr.post_id = g.post_id AND rr.element_id = e.id`+hiddenClause+`
		        ORDER BY rr.id DESC LIMIT 1)
		FROM game_elements ge
		JOIN games g ON g.id = ge.game_id
		JOIN elements e ON e.id = ge.element_id AND e.deleted_at IS NULL
		WHERE ge.game_id = ? AND ge.element_id IN (`+placeholders+`)`, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make(map[int64]GameResultItem, len(elementIDs))
	for rows.Next() {
		var item GameResultItem
		var globalRank sql.NullInt64
		if err := rows.Scan(
			&item.Element.ID, &item.Element.SourceURL, &item.Element.ThumbURL,
			&item.Element.MediumThumbURL, &item.Element.LowThumbURL,
			&item.Element.Title, &item.Element.Type, &item.Element.VideoStartSecond,
			&item.Element.VideoEndSecond, &item.Element.VideoSource, &item.Element.VideoID,
			&item.Element.VideoDurationSecond, &item.WinCount, &globalRank,
		); err != nil {
			return nil, err
		}
		item.GlobalRank = positiveRankPointer(globalRank)
		items[item.Element.ID] = item
	}
	return items, rows.Err()
}

func positiveRankPointer(rank sql.NullInt64) *int64 {
	if !rank.Valid || rank.Int64 <= 0 {
		return nil
	}
	value := rank.Int64
	return &value
}

/*
resultRankVisibilityClause builds the rank_reports row filter for whichever of the
soft-delete and moderation columns this database actually has.

THE COLUMN PAIR IS NOT STABLE ACROSS SCHEMA SNAPSHOTS. A database restored from one era
has rank_reports.deleted_at and no hidden; another has hidden and no deleted_at. This
query used to name deleted_at outright and probe only for hidden, so on a
hidden-only database every finished game's result answered
"Unknown column 'rr.deleted_at' in 'where clause'" — the whole endpoint, not one row.

Mirrors publiccontent's probe of the same table, and costs one information_schema query
per process, cached behind capabilityMu.
*/
func (repository *MySQLRepository) resultRankVisibilityClause(ctx context.Context) (string, error) {
	repository.capabilityMu.RLock()
	cached := repository.rankVisibility
	repository.capabilityMu.RUnlock()
	if cached != nil {
		return *cached, nil
	}

	rows, err := repository.database.QueryContext(ctx, `
		SELECT column_name FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = 'rank_reports'
		  AND column_name IN ('deleted_at', 'hidden')`)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var hasDeletedAt, hasHidden bool
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return "", err
		}
		switch column {
		case "deleted_at":
			hasDeletedAt = true
		case "hidden":
			hasHidden = true
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	clause := resultRankVisibility(hasDeletedAt, hasHidden)
	repository.capabilityMu.Lock()
	repository.rankVisibility = &clause
	repository.capabilityMu.Unlock()
	return clause, nil
}

// resultRankVisibility assembles the filter in a fixed order, so the generated SQL is the
// same however information_schema orders its rows. A column this database does not have
// contributes no filter rather than an error.
func resultRankVisibility(hasDeletedAt, hasHidden bool) string {
	clause := ""
	if hasDeletedAt {
		clause += " AND rr.deleted_at IS NULL"
	}
	if hasHidden {
		clause += " AND rr.hidden = 0"
	}
	return clause
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func queryRandomElements(ctx context.Context, queryer queryer, postID int64, limit int) ([]Element, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT e.id, e.source_url, e.thumb_url, e.mediumthumb_url, e.lowthumb_url,
		       e.title, e.type, e.video_start_second, e.video_end_second,
		       e.video_source, e.video_id, e.video_duration_second
		FROM post_elements pe
		JOIN elements e ON e.id = pe.element_id AND e.deleted_at IS NULL
		WHERE pe.post_id = ?
		ORDER BY RAND()
		LIMIT ?`, postID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	elements := make([]Element, 0, limit)
	for rows.Next() {
		var element Element
		if err := rows.Scan(
			&element.ID, &element.SourceURL, &element.ThumbURL, &element.MediumThumbURL,
			&element.LowThumbURL, &element.Title, &element.Type,
			&element.VideoStartSecond, &element.VideoEndSecond, &element.VideoSource,
			&element.VideoID, &element.VideoDurationSecond,
		); err != nil {
			return nil, err
		}
		elements = append(elements, element)
	}
	return elements, rows.Err()
}

func queryGameElements(ctx context.Context, queryer queryer, gameID int64) ([]Element, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT e.id, e.source_url, e.thumb_url, e.mediumthumb_url, e.lowthumb_url,
		       e.title, e.type, e.video_start_second, e.video_end_second,
		       e.video_source, e.video_id, e.video_duration_second
		FROM game_elements ge
		JOIN elements e ON e.id = ge.element_id AND e.deleted_at IS NULL
		WHERE ge.game_id = ?
		ORDER BY ge.id`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	elements := make([]Element, 0)
	for rows.Next() {
		var element Element
		if err := rows.Scan(
			&element.ID, &element.SourceURL, &element.ThumbURL, &element.MediumThumbURL,
			&element.LowThumbURL, &element.Title, &element.Type,
			&element.VideoStartSecond, &element.VideoEndSecond, &element.VideoSource,
			&element.VideoID, &element.VideoDurationSecond,
		); err != nil {
			return nil, err
		}
		elements = append(elements, element)
	}
	return elements, rows.Err()
}

type persistedRound struct {
	CurrentRound   int
	OfRound        int
	RemainElements int
	WinnerID       int64
	LoserID        int64
}

type gameElementState struct {
	rowID         int64
	elementID     int64
	title         string
	winCount      int
	eliminated    bool
	ready         bool
	originalWins  int
	originalOut   bool
	originalReady bool
}

func (repository *MySQLRepository) SubmitVotes(ctx context.Context, gameSerial string, input BatchInput) (BatchResult, error) {
	visible, visibleArguments := postaccess.VisibilityClause("p", "pp", input.Caller)
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return BatchResult{}, err
	}
	defer transaction.Rollback()

	var gameID, postID int64
	var elementCount int
	var completedAt sql.NullTime
	var isCensored bool
	if err := transaction.QueryRowContext(ctx, `
		SELECT g.id, g.post_id, g.element_count, g.completed_at, p.is_censored
		FROM games g
		JOIN posts p ON p.id = g.post_id AND p.deleted_at IS NULL
		JOIN post_policies pp ON pp.post_id = p.id AND `+visible+`
		WHERE g.serial = ?
		FOR UPDATE`, append(visibleArguments, gameSerial)...).Scan(
		&gameID, &postID, &elementCount, &completedAt, &isCensored); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BatchResult{}, ErrNotFound
		}
		return BatchResult{}, err
	}
	if err := repository.adult.RequireSignIn(isCensored, input.Caller); err != nil {
		return BatchResult{}, err
	}

	rounds, err := loadRounds(ctx, transaction, gameID)
	if err != nil {
		return BatchResult{}, err
	}
	serverVoteCount := len(rounds)
	if input.ExpectedVoteCount > serverVoteCount {
		return BatchResult{}, conflict("revision_mismatch", serverVoteCount)
	}
	committed := rounds[input.ExpectedVoteCount:]
	if len(committed) > len(input.Votes) {
		return BatchResult{}, conflict("revision_mismatch", serverVoteCount)
	}
	for index, round := range committed {
		vote := input.Votes[index]
		if round.WinnerID != vote.WinnerID || round.LoserID != vote.LoserID {
			return BatchResult{}, conflict("revision_mismatch", serverVoteCount)
		}
	}
	pendingVotes := input.Votes[len(committed):]

	states, err := loadElementStates(ctx, transaction, gameID)
	if err != nil {
		return BatchResult{}, err
	}
	if len(states) == 0 && len(pendingVotes) > 0 {
		return BatchResult{}, conflict("game_state_unavailable", serverVoteCount)
	}

	stage, remain, matchIndex, matchesInStage := batchPosition(rounds, elementCount)
	now := repository.now()
	insertRound, err := transaction.PrepareContext(ctx, `
		INSERT INTO game_1v1_rounds
		(game_id, current_round, of_round, remain_elements, winner_id, loser_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return BatchResult{}, err
	}
	defer insertRound.Close()

	var finalVote Vote
	// Collected as the rounds are written, not reconstructed afterwards: these numbers
	// are what a room's wagers are matched on.
	settled := make([]SettledRound, 0, len(pendingVotes))
	for _, vote := range pendingVotes {
		winner, winnerExists := states[vote.WinnerID]
		loser, loserExists := states[vote.LoserID]
		if !winnerExists || !loserExists || vote.WinnerID == vote.LoserID {
			return BatchResult{}, conflict("element_not_in_game", serverVoteCount)
		}
		if winner.eliminated {
			return BatchResult{}, conflict("winner_eliminated", serverVoteCount)
		}
		if loser.eliminated {
			return BatchResult{}, conflict("loser_eliminated", serverVoteCount)
		}

		winner.winCount++
		winner.ready = false
		loser.eliminated = true
		loser.ready = false
		remain--
		matchIndex++
		if matchIndex > matchesInStage {
			stage++
			matchIndex = 1
			matchesInStage = matchesForStage(stage, remain+1)
		}
		if matchIndex == matchesInStage {
			for _, state := range states {
				if !state.eliminated {
					state.ready = true
				}
			}
		}
		if _, err := insertRound.ExecContext(
			ctx, gameID, matchIndex, matchesInStage, remain,
			vote.WinnerID, vote.LoserID, now, now,
		); err != nil {
			return BatchResult{}, err
		}
		settled = append(settled, SettledRound{
			WinnerID: vote.WinnerID, LoserID: vote.LoserID,
			CurrentRound: matchIndex, OfRound: matchesInStage, RemainElements: remain,
		})
		finalVote = vote
	}

	for _, state := range states {
		if state.winCount == state.originalWins && state.eliminated == state.originalOut && state.ready == state.originalReady {
			continue
		}
		if _, err := transaction.ExecContext(ctx, `
			UPDATE game_elements
			SET win_count = ?, is_eliminated = ?, is_ready = ?
			WHERE id = ?`, state.winCount, state.eliminated, state.ready, state.rowID); err != nil {
			return BatchResult{}, err
		}
	}
	if len(pendingVotes) > 0 {
		// candidates means "the pair on screen"; see BatchInput.CurrentCandidates. When
		// the client says what it is displaying, that is what goes in.
		//
		// When it does not, the last decided pair goes in — which is what this did before
		// rooms existed. Kept rather than left untouched because Laravel's
		// getCurrentElements reads this column for a game played through either stack, and
		// a stale value there is a worse failure than a merely unhelpful one. A solo game
		// has nobody reading it at all.
		candidates := fmt.Sprintf("%d,%d", finalVote.WinnerID, finalVote.LoserID)
		if pair, ok := onScreenPair(input.CurrentCandidates, states); ok {
			candidates = pair
		}
		if _, err := transaction.ExecContext(ctx, `
			UPDATE games SET vote_count = vote_count + ?, candidates = ?, updated_at = ? WHERE id = ?`,
			len(pendingVotes), candidates, now, gameID,
		); err != nil {
			return BatchResult{}, err
		}
	}

	complete := remain == 1 || completedAt.Valid
	// The transition, not the state. Laravel raises GameComplete once, inside this
	// branch, and its listeners flag the post's ranks as stale and rebuild the
	// report. A client retrying against an already-finished game reaches `complete`
	// but not this branch, and must not re-trigger those.
	justCompleted := remain == 1 && !completedAt.Valid
	if justCompleted {
		if len(pendingVotes) == 0 {
			if len(rounds) == 0 {
				return BatchResult{}, conflict("game_state_unavailable", serverVoteCount)
			}
			lastRound := rounds[len(rounds)-1]
			finalVote = Vote{WinnerID: lastRound.WinnerID, LoserID: lastRound.LoserID}
		}
		if _, err := transaction.ExecContext(ctx, `UPDATE games SET completed_at = ?, updated_at = ? WHERE id = ?`, now, now, gameID); err != nil {
			return BatchResult{}, err
		}
		winner := states[finalVote.WinnerID]
		loser := states[finalVote.LoserID]
		if winner == nil || loser == nil {
			return BatchResult{}, conflict("game_state_unavailable", serverVoteCount)
		}
		anonymousID := sql.NullString{String: input.AnonymousID, Valid: input.AnonymousID != ""}
		finalists, ok := finalPairAsDisplayed(input.CurrentCandidates, finalVote)
		if !ok {
			finalists = fmt.Sprintf("%d,%d", finalVote.WinnerID, finalVote.LoserID)
		}
		if _, err := transaction.ExecContext(ctx, `
			INSERT INTO user_game_results
			(user_id, anonymous_id, game_id, champion_id, loser_id, loser_name, champion_name, candidates, created_at, updated_at)
			SELECT NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?
			WHERE NOT EXISTS (SELECT 1 FROM user_game_results WHERE game_id = ?)`,
			anonymousID, gameID, finalVote.WinnerID, finalVote.LoserID,
			loser.title, winner.title, finalists, now, now, gameID,
		); err != nil {
			return BatchResult{}, err
		}
	}

	if err := transaction.Commit(); err != nil {
		return BatchResult{}, err
	}
	return BatchResult{
		Status:          status(complete),
		ServerVoteCount: serverVoteCount + len(pendingVotes),
		Complete:        complete,
		JustCompleted:   justCompleted,
		PostID:          postID,
		SettledRounds:   settled,
	}, nil
}

// onScreenPair validates the pair a host says it is displaying.
//
// Both elements must belong to the game and still be in play. A pair naming an eliminated
// element would put a room's participants on a match that can never be settled — the
// settlement matches on the pairing, so those wagers would sit unresolved forever. An
// invalid pair is ignored rather than rejected: the votes in the same request are already
// valid, and refusing them over a display hint would lose real progress.
func onScreenPair(candidates []int64, states map[int64]*gameElementState) (string, bool) {
	if len(candidates) != 2 || candidates[0] == candidates[1] {
		return "", false
	}
	for _, id := range candidates {
		state, exists := states[id]
		if !exists || state.eliminated {
			return "", false
		}
	}
	return fmt.Sprintf("%d,%d", candidates[0], candidates[1]), true
}

/*
finalPairAsDisplayed formats a completed game's last pair in the order the player saw
it, left first.

game_1v1_rounds records a winner and a loser, never a side, so a client that picks its
own pairs is the only thing that knows which finalist stood on the left. The home
page's champion rail places the two finalists with the column this feeds, so writing
the winner first for every game is exactly what makes the left candidate look like it
always wins.

The value is client-supplied and purely presentational, so it is accepted only when it
names the two elements this game actually ended on — anything else falls back to
winner-first rather than putting a stranger on the home page.
*/
func finalPairAsDisplayed(candidates []int64, finalVote Vote) (string, bool) {
	if len(candidates) != 2 {
		return "", false
	}
	left, right := candidates[0], candidates[1]
	sameElements := (left == finalVote.WinnerID && right == finalVote.LoserID) ||
		(left == finalVote.LoserID && right == finalVote.WinnerID)
	if !sameElements {
		return "", false
	}
	return fmt.Sprintf("%d,%d", left, right), true
}

func loadRounds(ctx context.Context, transaction *sql.Tx, gameID int64) ([]persistedRound, error) {
	return queryPersistedRounds(ctx, transaction, gameID)
}

func queryPersistedRounds(ctx context.Context, queryer queryer, gameID int64) ([]persistedRound, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT current_round, of_round, remain_elements, winner_id, loser_id
		FROM game_1v1_rounds WHERE game_id = ? ORDER BY id`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rounds := make([]persistedRound, 0)
	for rows.Next() {
		var round persistedRound
		if err := rows.Scan(&round.CurrentRound, &round.OfRound, &round.RemainElements, &round.WinnerID, &round.LoserID); err != nil {
			return nil, err
		}
		rounds = append(rounds, round)
	}
	return rounds, rows.Err()
}

func topResultElementIDs(rounds []persistedRound, limit int) []int64 {
	if limit <= 0 || len(rounds) == 0 {
		return nil
	}
	ordered := append([]persistedRound(nil), rounds...)
	sort.SliceStable(ordered, func(left, right int) bool {
		return ordered[left].RemainElements < ordered[right].RemainElements
	})
	if ordered[0].RemainElements != 1 {
		return nil
	}
	ids := make([]int64, 0, min(limit, len(ordered)+1))
	ids = append(ids, ordered[0].WinnerID)
	for _, round := range ordered {
		if len(ids) >= limit {
			break
		}
		ids = append(ids, round.LoserID)
	}
	return ids
}

func loadElementStates(ctx context.Context, transaction *sql.Tx, gameID int64) (map[int64]*gameElementState, error) {
	rows, err := transaction.QueryContext(ctx, `
		SELECT ge.id, ge.element_id, e.title, ge.win_count, ge.is_eliminated, ge.is_ready
		FROM game_elements ge
		JOIN elements e ON e.id = ge.element_id
		WHERE ge.game_id = ?
		FOR UPDATE`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	states := make(map[int64]*gameElementState)
	for rows.Next() {
		state := &gameElementState{}
		if err := rows.Scan(&state.rowID, &state.elementID, &state.title, &state.winCount, &state.eliminated, &state.ready); err != nil {
			return nil, err
		}
		state.originalWins = state.winCount
		state.originalOut = state.eliminated
		state.originalReady = state.ready
		states[state.elementID] = state
	}
	return states, rows.Err()
}

func batchPosition(rounds []persistedRound, elementCount int) (stage, remain, matchIndex, matchesInStage int) {
	if len(rounds) == 0 {
		return 1, elementCount, 0, matchesForStage(1, elementCount)
	}
	stageCount := 0
	for _, round := range rounds {
		if round.CurrentRound == 1 {
			stageCount++
		}
	}
	last := rounds[len(rounds)-1]
	if last.CurrentRound >= last.OfRound {
		stage = stageCount + 1
		return stage, last.RemainElements, 0, matchesForStage(stage, last.RemainElements)
	}
	stage = max(stageCount, 1)
	return stage, last.RemainElements, last.CurrentRound, last.OfRound
}

func matchesForStage(stage, remain int) int {
	if remain <= 1 {
		return 0
	}
	if stage == 1 {
		return (remain + 1) / 2
	}
	if stage == 2 {
		power := 1
		for power*2 <= remain {
			power *= 2
		}
		if difference := remain - power; difference > 0 {
			return difference
		}
	}
	return remain / 2
}

func conflict(reason string, serverVoteCount int) error {
	return &ConflictError{Reason: reason, ServerVoteCount: serverVoteCount}
}

func status(complete bool) string {
	if complete {
		return "end_game"
	}
	return "processing"
}

func newUUID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16]), nil
}
