package comments

import (
	"context"
	"crypto/rand"
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

func (repository *MySQLRepository) List(ctx context.Context, postSerial string, page, perPage int, viewer Viewer) (Page, error) {
	postID, err := repository.postID(ctx, postSerial)
	if err != nil {
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

	rows, err := repository.database.QueryContext(ctx, `
		SELECT c.id, c.content, c.created_at, c.edited_at, c.nickname,
		       c.anonymous_mode, u.avatar_url, c.label
		FROM post_comments pc
		JOIN comments c ON c.id = pc.comment_id
		LEFT JOIN users u ON u.id = c.user_id
		WHERE pc.post_id = ? AND c.deleted_at IS NULL
		ORDER BY c.id DESC
		LIMIT ? OFFSET ?`, postID, perPage, (page-1)*perPage)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()

	items := make([]Comment, 0, perPage)
	for rows.Next() {
		var item Comment
		var createdAt time.Time
		var editedAt sql.NullTime
		var avatarURL sql.NullString
		var rawLabel sql.NullString
		var anonymousMode bool
		if err := rows.Scan(
			&item.ID, &item.Content, &createdAt, &editedAt, &item.Nickname,
			&anonymousMode, &avatarURL, &rawLabel,
		); err != nil {
			return Page{}, err
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
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
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}

	profile, err := repository.profile(ctx, postID, viewer)
	if err != nil {
		return Page{}, err
	}
	totalPages := 0
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(perPage)))
	}
	return Page{
		Items: items, Page: page, PerPage: perPage, Total: total, TotalPages: totalPages, Profile: profile,
	}, nil
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
	deleteHash, err := randomHash()
	if err != nil {
		return Comment{}, err
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
		(user_id, content, anonymous_id, nickname, label, anonymous_mode, delete_hash, ip, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.Viewer.UserID, input.Content, anonymousID, nickname, string(label), input.Anonymous, deleteHash, ip, now, now,
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
	if err := transaction.Commit(); err != nil {
		return Comment{}, err
	}

	if input.Anonymous {
		nickname = anonymousNickname
		avatarURL = nil
	}
	return Comment{
		ID: commentID, Content: input.Content, CreatedAt: now.Format(time.RFC3339),
		Nickname: nickname, AvatarURL: avatarURL, Champions: champions,
	}, nil
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
