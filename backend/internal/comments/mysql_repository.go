package comments

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

const anonymousNickname = "Anonymous"

type MySQLRepository struct {
	database *sql.DB
	now      func() time.Time
}

func NewMySQLRepository(database *sql.DB) *MySQLRepository {
	return &MySQLRepository{database: database, now: time.Now}
}

// List returns one page of a post's comment tree.
//
// The unit of a page is the floor, not the comment: it carries the page's top-level
// comments and every reply hanging off them, ordered for display — newest floor first,
// and within a floor its replies depth-first, oldest first. The array stays flat and the
// client rebuilds the tree from parent_id, which leaves the page envelope the shape it
// had before replies existed.
func (repository *MySQLRepository) List(ctx context.Context, postSerial string, page, perPage int, viewer Viewer) (Page, error) {
	postID, err := repository.postID(ctx, postSerial)
	if err != nil {
		return Page{}, err
	}

	// Two counts, because they answer different questions. Floors decide how many pages
	// there are and include tombstones, since a deleted comment keeps its floor and its
	// replies. The total beside the heading is what a reader can actually read.
	var floors int64
	if err := repository.database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM post_comments pc
		JOIN comments c ON c.id = pc.comment_id
		WHERE pc.post_id = ? AND c.parent_id IS NULL`, postID).Scan(&floors); err != nil {
		return Page{}, err
	}
	var total int64
	if err := repository.database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM post_comments pc
		JOIN comments c ON c.id = pc.comment_id
		WHERE pc.post_id = ? AND c.deleted_at IS NULL`, postID).Scan(&total); err != nil {
		return Page{}, err
	}

	floorRefs, err := repository.floorPage(ctx, postID, page, perPage)
	if err != nil {
		return Page{}, err
	}
	items, err := repository.threads(ctx, floorRefs, viewer)
	if err != nil {
		return Page{}, err
	}

	profile, err := repository.profile(ctx, postID, viewer)
	if err != nil {
		return Page{}, err
	}
	totalPages := 0
	if floors > 0 {
		totalPages = int(math.Ceil(float64(floors) / float64(perPage)))
	}
	return Page{
		Items: items, Page: page, PerPage: perPage, Total: total, TotalPages: totalPages, Profile: profile,
	}, nil
}

// floorRef is a top-level comment and the floor number it sits on.
type floorRef struct {
	id    int64
	floor int
}

// floorPage numbers the post's floors and returns the requested page of them.
//
// The number is counted at read time rather than stored. It is the position of the
// comment among the post's top-level comments ordered by id, and that position is fixed:
// deleting keeps the row, and a comment is never re-parented. Counting also means the
// legacy PHP endpoint, which knows nothing of any of this, still lands its inserts on the
// next floor. The subquery scans one post's top-level comments — 774 rows for the most
// commented post in the database, around 19 for the average one.
func (repository *MySQLRepository) floorPage(ctx context.Context, postID int64, page, perPage int) ([]floorRef, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, floor
		FROM (
			SELECT c.id AS id, ROW_NUMBER() OVER (ORDER BY c.id) AS floor
			FROM post_comments pc
			JOIN comments c ON c.id = pc.comment_id
			WHERE pc.post_id = ? AND c.parent_id IS NULL
		) numbered
		ORDER BY id DESC
		LIMIT ? OFFSET ?`, postID, perPage, (page-1)*perPage)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]floorRef, 0, perPage)
	for rows.Next() {
		var ref floorRef
		if err := rows.Scan(&ref.id, &ref.floor); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

// threads reads the given floors with their replies and flattens them into display order.
func (repository *MySQLRepository) threads(ctx context.Context, floors []floorRef, viewer Viewer) ([]Comment, error) {
	items := make([]Comment, 0, len(floors))
	if len(floors) == 0 {
		return items, nil
	}
	arguments := make([]any, 0, len(floors))
	for _, floor := range floors {
		arguments = append(arguments, floor.id)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(floors)), ",")

	rows, err := repository.database.QueryContext(ctx, `
		WITH RECURSIVE thread (id) AS (
			SELECT id FROM comments WHERE id IN (`+placeholders+`)
			UNION ALL
			SELECT c.id FROM comments c JOIN thread t ON c.parent_id = t.id
		)
		SELECT c.id, c.parent_id, c.depth, c.content, c.created_at, c.edited_at, c.nickname,
		       c.anonymous_mode, u.avatar_url, c.label, c.deleted_at, c.user_id, c.delete_hash
		FROM thread t
		JOIN comments c ON c.id = t.id
		LEFT JOIN users u ON u.id = c.user_id
		ORDER BY c.id`, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := make(map[int64]Comment, len(floors))
	children := make(map[int64][]int64, len(floors))
	for rows.Next() {
		var (
			item          Comment
			parentID      sql.NullInt64
			createdAt     time.Time
			editedAt      sql.NullTime
			avatarURL     sql.NullString
			rawLabel      sql.NullString
			deletedAt     sql.NullTime
			userID        sql.NullInt64
			deleteHash    string
			anonymousMode bool
			content       string
			nickname      string
		)
		if err := rows.Scan(
			&item.ID, &parentID, &item.Depth, &content, &createdAt, &editedAt, &nickname,
			&anonymousMode, &avatarURL, &rawLabel, &deletedAt, &userID, &deleteHash,
		); err != nil {
			return nil, err
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		item.Champions = []string{}
		if parentID.Valid {
			parent := parentID.Int64
			item.ParentID = &parent
			children[parent] = append(children[parent], item.ID)
		}
		// A tombstone carries an id, a timestamp and, for a floor, its number. Nothing
		// else: the author and the text are what deletion was asked for, so they are
		// dropped here rather than being sent for the client to agree not to render.
		if deletedAt.Valid {
			item.Deleted = true
		} else {
			item.Content = content
			item.Nickname = nickname
			if editedAt.Valid {
				formatted := editedAt.Time.Format(time.RFC3339)
				item.EditedAt = &formatted
			}
			if anonymousMode {
				item.Nickname = anonymousNickname
			} else if avatarURL.Valid {
				item.AvatarURL = &avatarURL.String
			}
			item.Champions = championsFromLabel(rawLabel.String)
			item.CanDelete = ownedBy(viewer, userID, deleteHash)
		}
		nodes[item.ID] = item
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, floor := range floors {
		thread, _ := collectThread(nodes, children, floor.id)
		if len(thread) == 0 {
			continue
		}
		// A floor's tombstone is kept whatever is under it: preserving the numbering is
		// the reason the row was not simply removed.
		number := floor.floor
		thread[0].Floor = &number
		items = append(items, thread...)
	}
	return items, nil
}

// collectThread flattens one comment and its replies, and reports whether anything in the
// result is still readable.
//
// A deleted reply is dropped unless a reply beneath it survives — the tombstone is only
// there to hold the shape of the thread together, and a run of them holding up nothing
// would be a graveyard where a conversation used to be. The caller keeps a deleted floor
// regardless, because its number is worth holding on to.
func collectThread(nodes map[int64]Comment, children map[int64][]int64, id int64) ([]Comment, bool) {
	node, ok := nodes[id]
	if !ok {
		return nil, false
	}
	thread := []Comment{node}
	readable := !node.Deleted
	for _, childID := range children[id] {
		branch, branchReadable := collectThread(nodes, children, childID)
		if !branchReadable {
			continue
		}
		thread = append(thread, branch...)
		readable = true
	}
	return thread, readable
}

// ownedBy decides whether the viewer may delete a comment.
//
// An account owns what it posted, including what it posted anonymously — the row still
// carries the user id. A signed-out commenter owns what their delete-key cookie hashes to,
// and only on rows no account owns, so signing in later never takes a comment away from
// the account that wrote it.
func ownedBy(viewer Viewer, userID sql.NullInt64, deleteHash string) bool {
	if viewer.UserID != nil && userID.Valid && userID.Int64 == *viewer.UserID {
		return true
	}
	if userID.Valid || viewer.DeleteHash == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(viewer.DeleteHash), []byte(deleteHash)) == 1
}

func (repository *MySQLRepository) Create(ctx context.Context, postSerial string, input CreateInput) (Comment, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return Comment{}, err
	}
	defer transaction.Rollback()

	postID, err := postIDWith(ctx, transaction, postSerial)
	if err != nil {
		return Comment{}, err
	}
	now := repository.now()
	if err := enforceCreateRateLimit(ctx, transaction, input.Viewer.UserID, input.IP, now); err != nil {
		return Comment{}, err
	}
	parentID, depth, err := resolveParent(ctx, transaction, postID, input.ParentID)
	if err != nil {
		return Comment{}, err
	}

	nickname := anonymousNickname
	var avatarURL *string
	if input.Viewer.UserID != nil {
		var avatar sql.NullString
		if err := transaction.QueryRowContext(ctx, `SELECT name, avatar_url FROM users WHERE id = ?`, *input.Viewer.UserID).Scan(&nickname, &avatar); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return Comment{}, ErrNotFound
			}
			return Comment{}, err
		}
		nickname = truncate(nickname, 30)
		if avatar.Valid {
			avatarURL = &avatar.String
		}
	}
	champions, err := championsForViewer(ctx, transaction, postID, input.Viewer)
	if err != nil {
		return Comment{}, err
	}
	label, err := json.Marshal(map[string][]string{"champions": champions})
	if err != nil {
		return Comment{}, err
	}
	// The row's delete key. Historically a value nobody kept; now the hash of the
	// browser's delete-key cookie, which is what lets a signed-out commenter take their
	// own comment down. A caller that minted no key falls back to the old behaviour
	// rather than to an empty string: an unguessable value makes the row undeletable,
	// where a shared one would make it deletable from every key-less browser at once.
	deleteHash := input.Viewer.DeleteHash
	if deleteHash == "" {
		if deleteHash, err = randomHash(); err != nil {
			return Comment{}, err
		}
	}
	anonymousID := strings.TrimSpace(input.AnonymousID)
	if anonymousID == "" {
		anonymousID = "unknown"
	}
	ip := strings.TrimSpace(input.IP)
	if ip == "" {
		ip = "unknown"
	}
	result, err := transaction.ExecContext(ctx, `
		INSERT INTO comments
		(user_id, parent_id, depth, content, anonymous_id, nickname, label, anonymous_mode, delete_hash, ip, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.Viewer.UserID, parentID, depth, input.Content, anonymousID, nickname, string(label), input.Anonymous, deleteHash, ip, now, now,
	)
	if err != nil {
		return Comment{}, err
	}
	commentID, err := result.LastInsertId()
	if err != nil {
		return Comment{}, err
	}
	if _, err := transaction.ExecContext(ctx, `INSERT INTO post_comments (post_id, comment_id) VALUES (?, ?)`, postID, commentID); err != nil {
		return Comment{}, err
	}
	var floor *int
	if parentID == nil {
		var number int
		if err := transaction.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM post_comments pc
			JOIN comments c ON c.id = pc.comment_id
			WHERE pc.post_id = ? AND c.parent_id IS NULL`, postID).Scan(&number); err != nil {
			return Comment{}, err
		}
		floor = &number
	}
	if err := transaction.Commit(); err != nil {
		return Comment{}, err
	}

	if input.Anonymous {
		nickname = anonymousNickname
		avatarURL = nil
	}
	return Comment{
		ID: commentID, ParentID: parentID, Depth: depth, Floor: floor,
		Content: input.Content, CreatedAt: now.Format(time.RFC3339),
		Nickname: nickname, AvatarURL: avatarURL, Champions: champions, CanDelete: true,
	}, nil
}

// resolveParent validates a reply target and returns the depth its reply belongs at.
func resolveParent(ctx context.Context, transaction *sql.Tx, postID int64, parentID *int64) (*int64, int, error) {
	if parentID == nil {
		return nil, 1, nil
	}
	var parentDepth int
	err := transaction.QueryRowContext(ctx, `
		SELECT c.depth
		FROM comments c
		JOIN post_comments pc ON pc.comment_id = c.id
		WHERE c.id = ? AND pc.post_id = ? AND c.deleted_at IS NULL`, *parentID, postID).Scan(&parentDepth)
	if errors.Is(err, sql.ErrNoRows) {
		// Absent, deleted, or on another post — one answer for all three. It is
		// deliberately not ErrNotFound: the post was found, the reply target is what
		// the caller got wrong.
		return nil, 0, ErrInvalidParent
	}
	if err != nil {
		return nil, 0, err
	}
	if parentDepth >= MaxDepth {
		return nil, 0, ErrInvalidParent
	}
	return parentID, parentDepth + 1, nil
}

// Delete turns a comment into a tombstone.
//
// Soft, and not only to keep the row recoverable: the floor numbers are counted from the
// rows that exist, and the replies underneath point at this one. Removing it would
// renumber every floor above it and orphan its thread.
func (repository *MySQLRepository) Delete(ctx context.Context, postSerial string, commentID int64, viewer Viewer) error {
	postID, err := repository.postID(ctx, postSerial)
	if err != nil {
		return err
	}
	claims := make([]string, 0, 2)
	arguments := []any{repository.now(), repository.now(), commentID, postID}
	if viewer.UserID != nil {
		claims = append(claims, "c.user_id = ?")
		arguments = append(arguments, *viewer.UserID)
	}
	if viewer.DeleteHash != "" {
		claims = append(claims, "(c.user_id IS NULL AND c.delete_hash = ?)")
		arguments = append(arguments, viewer.DeleteHash)
	}
	if len(claims) == 0 {
		return ErrNotFound
	}
	result, err := repository.database.ExecContext(ctx, `
		UPDATE comments c
		JOIN post_comments pc ON pc.comment_id = c.id
		SET c.deleted_at = ?, c.updated_at = ?
		WHERE c.id = ? AND pc.post_id = ? AND c.deleted_at IS NULL AND (`+strings.Join(claims, " OR ")+`)`,
		arguments...,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		// One answer for "no such comment", "not yours" and "already gone", so the
		// endpoint cannot be used to find out which comment ids exist or who wrote them.
		return ErrNotFound
	}
	return nil
}

func (repository *MySQLRepository) Report(ctx context.Context, postSerial string, commentID int64, input ReportInput) error {
	postID, err := repository.postID(ctx, postSerial)
	if err != nil {
		return err
	}
	var content string
	if err := repository.database.QueryRowContext(ctx, `
		SELECT c.content
		FROM comments c
		JOIN post_comments pc ON pc.comment_id = c.id
		WHERE c.id = ? AND pc.post_id = ? AND c.deleted_at IS NULL`, commentID, postID).Scan(&content); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	_, err = repository.database.ExecContext(ctx, `
		INSERT INTO reported_comments
		(comment_id, reporter_id, reporter_ip, comment_content, reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		commentID, input.Viewer.UserID, input.IP, content, input.Reason, repository.now(), repository.now(),
	)
	return err
}

func (repository *MySQLRepository) postID(ctx context.Context, postSerial string) (int64, error) {
	return postIDWith(ctx, repository.database, postSerial)
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func postIDWith(ctx context.Context, query queryRower, postSerial string) (int64, error) {
	var postID int64
	err := query.QueryRowContext(ctx, `
		SELECT p.id
		FROM posts p
		JOIN post_policies pp ON pp.post_id = p.id AND pp.access_policy = 'public'
		WHERE p.serial = ? AND p.deleted_at IS NULL
		LIMIT 1`, postSerial).Scan(&postID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return postID, err
}

func (repository *MySQLRepository) profile(ctx context.Context, postID int64, viewer Viewer) (Profile, error) {
	profile := Profile{Nickname: anonymousNickname, Champions: []string{}}
	if viewer.UserID != nil {
		var avatar sql.NullString
		if err := repository.database.QueryRowContext(ctx, `SELECT name, avatar_url FROM users WHERE id = ?`, *viewer.UserID).Scan(&profile.Nickname, &avatar); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return Profile{}, err
			}
		} else {
			profile.Nickname = truncate(profile.Nickname, 30)
			profile.IsAuthenticated = true
			if avatar.Valid {
				profile.AvatarURL = &avatar.String
			}
		}
	}
	champions, err := championsForViewer(ctx, repository.database, postID, viewer)
	if err != nil {
		return Profile{}, err
	}
	profile.Champions = champions
	return profile, nil
}

type rowsQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func championsForViewer(ctx context.Context, query rowsQuerier, postID int64, viewer Viewer) ([]string, error) {
	where := "ugr.user_id IS NULL AND ugr.anonymous_id = ?"
	identifier := any(viewer.AnonymousID)
	if viewer.UserID != nil {
		where = "ugr.user_id = ?"
		identifier = *viewer.UserID
	}
	rows, err := query.QueryContext(ctx, `
		SELECT ugr.champion_name
		FROM user_game_results ugr
		JOIN games g ON g.id = ugr.game_id
		WHERE g.post_id = ? AND `+where+`
		ORDER BY ugr.id DESC
		LIMIT 1`, postID, identifier)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	champions := make([]string, 0, 1)
	for rows.Next() {
		var champion string
		if err := rows.Scan(&champion); err != nil {
			return nil, err
		}
		champions = append(champions, champion)
	}
	return champions, rows.Err()
}

func enforceCreateRateLimit(ctx context.Context, transaction *sql.Tx, userID *int64, ip string, now time.Time) error {
	where := "user_id IS NULL AND ip = ?"
	identifier := any(ip)
	limit := int64(3)
	if userID != nil {
		where = "user_id = ?"
		identifier = *userID
		limit = 5
	}
	var count int64
	if err := transaction.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM comments WHERE `+where+` AND created_at >= ?`, identifier, now.Add(-time.Minute),
	).Scan(&count); err != nil {
		return err
	}
	if count >= limit {
		return ErrRateLimit
	}
	return nil
}

func championsFromLabel(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var label struct {
		Champions []string `json:"champions"`
	}
	if err := json.Unmarshal([]byte(raw), &label); err != nil || label.Champions == nil {
		return []string{}
	}
	return label.Champions
}

func truncate(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	runes := []rune(value)
	return string(runes[:maximum])
}

func randomHash() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
