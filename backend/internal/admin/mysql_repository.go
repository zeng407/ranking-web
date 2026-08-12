package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MySQLRepository implements Store.
//
// Nothing here carries an ownership predicate, which is exactly what makes it the
// moderation repository: every statement reads or writes rows regardless of who they
// belong to. Authorization is the HTTP boundary's job — see the package comment.
type MySQLRepository struct {
	database *sql.DB
	now      func() time.Time
}

func NewMySQLRepository(database *sql.DB) *MySQLRepository {
	return &MySQLRepository{database: database, now: time.Now}
}

const postOwnerQuery = `
	SELECT user_id FROM posts WHERE serial = ? AND deleted_at IS NULL LIMIT 1`

func (repository *MySQLRepository) PostOwner(ctx context.Context, serial string) (int64, error) {
	var ownerID int64
	err := repository.database.QueryRowContext(ctx, postOwnerQuery, serial).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("admin: read the owner of post %q: %w", serial, err)
	}
	return ownerID, nil
}

// elementOwnerQuery resolves an element to the owner of the post it sits in.
//
// The lowest post id when an element sits in more than one, which the pivot allows: that
// is the row the authoring package's own join would have matched, so the edit that follows
// lands on the same post either way.
const elementOwnerQuery = `
	SELECT p.user_id
	  FROM post_elements AS pe
	  JOIN posts AS p ON p.id = pe.post_id AND p.deleted_at IS NULL
	  JOIN elements AS e ON e.id = pe.element_id AND e.deleted_at IS NULL
	 WHERE pe.element_id = ?
	 ORDER BY p.id
	 LIMIT 1`

func (repository *MySQLRepository) ElementOwner(ctx context.Context, elementID int64) (int64, error) {
	var ownerID int64
	err := repository.database.QueryRowContext(ctx, elementOwnerQuery, elementID).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("admin: read the owner of element %d: %w", elementID, err)
	}
	return ownerID, nil
}

const adminPostsPageQuery = `
	SELECT p.serial,
	       COALESCE(p.title, ''),
	       COALESCE(p.description, ''),
	       pp.access_policy,
	       p.is_censored,
	       COALESCE(all_time.play_count, 0),
	       p.user_id,
	       COALESCE(u.name, ''),
	       COALESCE(u.email, ''),
	       p.created_at
	  FROM posts AS p
	  JOIN post_policies AS pp ON pp.post_id = p.id
	  LEFT JOIN users AS u ON u.id = p.user_id
	  LEFT JOIN post_statistics AS all_time
	         ON all_time.post_id = p.id AND all_time.time_range = 'all'
	 WHERE p.deleted_at IS NULL
	 ORDER BY p.id DESC
	 LIMIT ? OFFSET ?`

const adminPostsCountQuery = `SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL`

func (repository *MySQLRepository) ListPosts(
	ctx context.Context, page, perPage int,
) ([]Post, int, error) {
	rows, err := repository.database.QueryContext(ctx, adminPostsPageQuery,
		perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("admin: list posts: %w", err)
	}
	defer rows.Close()

	posts := make([]Post, 0, perPage)
	for rows.Next() {
		var (
			post      Post
			createdAt sql.NullTime
		)
		if err := rows.Scan(&post.Serial, &post.Title, &post.Description, &post.AccessPolicy,
			&post.Censored, &post.PlayCount, &post.OwnerID, &post.OwnerName, &post.OwnerEmail,
			&createdAt); err != nil {
			return nil, 0, fmt.Errorf("admin: scan post: %w", err)
		}
		if createdAt.Valid {
			post.CreatedAt = createdAt.Time.Format(time.RFC3339)
		}
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("admin: list posts: %w", err)
	}

	var total int
	if err := repository.database.QueryRowContext(ctx, adminPostsCountQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("admin: count posts: %w", err)
	}
	return posts, total, nil
}

const setPostCensoredStatement = `
	UPDATE posts SET is_censored = ?, updated_at = ?
	 WHERE serial = ? AND deleted_at IS NULL`

func (repository *MySQLRepository) SetPostCensored(
	ctx context.Context, serial string, censored bool,
) error {
	// A statement that matches nothing is not an error here: either the post is gone —
	// which the caller's own read of it surfaces — or the flag already held that value.
	if _, err := repository.database.ExecContext(ctx, setPostCensoredStatement,
		censored, repository.now().UTC(), serial); err != nil {
		return fmt.Errorf("admin: set is_censored on post %q: %w", serial, err)
	}
	return nil
}

// usersPageQuery reads a page of accounts with the number of posts each still has.
//
// LIKE on two columns is the original's search. The wildcards are bound as part of the
// value rather than built into the SQL, so a keyword containing % matches literally only
// insofar as MySQL's LIKE does — the escaping that makes that true is in escapeLike.
const usersPageQuery = `
	SELECT u.id,
	       COALESCE(u.name, ''),
	       COALESCE(u.email, ''),
	       COALESCE(u.avatar_url, ''),
	       (SELECT COUNT(*) FROM posts AS p WHERE p.user_id = u.id AND p.deleted_at IS NULL),
	       u.created_at
	  FROM users AS u
	 WHERE (? = '' OR u.name LIKE ? ESCAPE '|' OR u.email LIKE ? ESCAPE '|')
	 ORDER BY u.id DESC
	 LIMIT ? OFFSET ?`

const usersCountQuery = `
	SELECT COUNT(*)
	  FROM users AS u
	 WHERE (? = '' OR u.name LIKE ? ESCAPE '|' OR u.email LIKE ? ESCAPE '|')`

const rolesForUsersQuery = `
	SELECT ur.user_id, r.slug
	  FROM user_roles AS ur
	  JOIN roles AS r ON r.id = ur.role_id
	 WHERE ur.user_id IN (%s)
	 ORDER BY r.id`

func (repository *MySQLRepository) ListUsers(
	ctx context.Context, keyword string, page, perPage int,
) ([]User, int, error) {
	pattern := "%" + escapeLike(keyword) + "%"

	rows, err := repository.database.QueryContext(ctx, usersPageQuery,
		keyword, pattern, pattern, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("admin: list users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0, perPage)
	for rows.Next() {
		var (
			user      User
			createdAt sql.NullTime
		)
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.AvatarURL,
			&user.PostCount, &createdAt); err != nil {
			return nil, 0, fmt.Errorf("admin: scan user: %w", err)
		}
		if createdAt.Valid {
			user.CreatedAt = createdAt.Time.Format(time.RFC3339)
		}
		user.Roles = []string{}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("admin: list users: %w", err)
	}

	if err := repository.attachRoles(ctx, users); err != nil {
		return nil, 0, err
	}

	var total int
	if err := repository.database.QueryRowContext(ctx, usersCountQuery,
		keyword, pattern, pattern).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("admin: count users: %w", err)
	}
	return users, total, nil
}

// attachRoles reads the whole page's roles in one round trip, because the list draws a
// "banned" badge on every row and one query per row is what made the original's admin
// screens slow.
func (repository *MySQLRepository) attachRoles(ctx context.Context, users []User) error {
	if len(users) == 0 {
		return nil
	}
	placeholders := make([]string, len(users))
	arguments := make([]any, len(users))
	byID := make(map[int64]int, len(users))
	for index := range users {
		placeholders[index] = "?"
		arguments[index] = users[index].ID
		byID[users[index].ID] = index
	}

	rows, err := repository.database.QueryContext(ctx,
		fmt.Sprintf(rolesForUsersQuery, strings.Join(placeholders, ", ")), arguments...)
	if err != nil {
		return fmt.Errorf("admin: read roles: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			userID int64
			slug   string
		)
		if err := rows.Scan(&userID, &slug); err != nil {
			return fmt.Errorf("admin: scan role: %w", err)
		}
		if index, ok := byID[userID]; ok {
			users[index].Roles = append(users[index].Roles, slug)
		}
	}
	return rows.Err()
}

const userExistsQuery = `SELECT 1 FROM users WHERE id = ? LIMIT 1`

const rolesForUserQuery = `
	SELECT r.slug
	  FROM user_roles AS ur
	  JOIN roles AS r ON r.id = ur.role_id
	 WHERE ur.user_id = ?
	 ORDER BY r.id`

func (repository *MySQLRepository) UserRoles(ctx context.Context, userID int64) ([]string, error) {
	var exists int
	err := repository.database.QueryRowContext(ctx, userExistsQuery, userID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("admin: look up user %d: %w", userID, err)
	}

	rows, err := repository.database.QueryContext(ctx, rolesForUserQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("admin: read the roles of user %d: %w", userID, err)
	}
	defer rows.Close()

	roles := make([]string, 0, 2)
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, fmt.Errorf("admin: scan role: %w", err)
		}
		roles = append(roles, slug)
	}
	return roles, rows.Err()
}

const roleIDQuery = `SELECT id FROM roles WHERE slug = ? AND deleted_at IS NULL LIMIT 1`

// addRoleStatement is INSERT IGNORE so re-banning an already-banned account is a no-op
// rather than a duplicate-key error: user_roles' primary key is the pair.
const addRoleStatement = `
	INSERT IGNORE INTO user_roles (user_id, role_id, created_at, updated_at) VALUES (?, ?, ?, ?)`

const removeRoleStatement = `DELETE FROM user_roles WHERE user_id = ? AND role_id = ?`

func (repository *MySQLRepository) AddRole(ctx context.Context, userID int64, slug string) error {
	roleID, err := repository.roleID(ctx, slug)
	if err != nil {
		return err
	}
	now := repository.now().UTC()
	if _, err := repository.database.ExecContext(ctx, addRoleStatement,
		userID, roleID, now, now); err != nil {
		return fmt.Errorf("admin: add role %q to user %d: %w", slug, userID, err)
	}
	return nil
}

func (repository *MySQLRepository) RemoveRole(ctx context.Context, userID int64, slug string) error {
	roleID, err := repository.roleID(ctx, slug)
	if err != nil {
		return err
	}
	if _, err := repository.database.ExecContext(ctx, removeRoleStatement, userID, roleID); err != nil {
		return fmt.Errorf("admin: remove role %q from user %d: %w", slug, userID, err)
	}
	return nil
}

// roleID resolves a slug. A missing role is an error rather than a silent no-op: the
// slugs are seeded by migration, so their absence means the deployment is broken and a
// ban that quietly did nothing would be worse than a 500.
func (repository *MySQLRepository) roleID(ctx context.Context, slug string) (int64, error) {
	var roleID int64
	err := repository.database.QueryRowContext(ctx, roleIDQuery, slug).Scan(&roleID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("admin: the %q role is not seeded", slug)
	}
	if err != nil {
		return 0, fmt.Errorf("admin: look up the %q role: %w", slug, err)
	}
	return roleID, nil
}

// carouselColumns is shared by the list and the single-item read so the two cannot
// disagree about what a slide is.
const carouselColumns = `
	       id, position, type,
	       COALESCE(title, ''),
	       COALESCE(description, ''),
	       COALESCE(image_url, ''),
	       COALESCE(video_url, ''),
	       COALESCE(video_source, ''),
	       COALESCE(video_id, ''),
	       video_start_second, video_end_second, is_active`

// carouselItemsQuery orders the way the admin screen lists them: active slides first,
// then by position, which is HomeCarouselController::index's orderBy pair.
var carouselItemsQuery = `
	SELECT` + carouselColumns + `
	  FROM home_carousel_items
	 WHERE deleted_at IS NULL
	 ORDER BY is_active, position, id`

var carouselItemQuery = `
	SELECT` + carouselColumns + `
	  FROM home_carousel_items
	 WHERE id = ? AND deleted_at IS NULL
	 LIMIT 1`

func (repository *MySQLRepository) CarouselItems(ctx context.Context) ([]CarouselItem, error) {
	rows, err := repository.database.QueryContext(ctx, carouselItemsQuery)
	if err != nil {
		return nil, fmt.Errorf("admin: list carousel items: %w", err)
	}
	defer rows.Close()

	items := make([]CarouselItem, 0)
	for rows.Next() {
		item, err := scanCarouselItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *MySQLRepository) CarouselItem(ctx context.Context, itemID int64) (CarouselItem, error) {
	item, err := scanCarouselItem(repository.database.QueryRowContext(ctx, carouselItemQuery, itemID))
	if errors.Is(err, sql.ErrNoRows) {
		return CarouselItem{}, ErrNotFound
	}
	return item, err
}

const createCarouselStatement = `
	INSERT INTO home_carousel_items
	       (position, type, title, description, image_url, video_url, video_source, video_id,
	        video_start_second, video_end_second, is_active, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func (repository *MySQLRepository) CreateCarouselItem(
	ctx context.Context, item CarouselItem,
) (CarouselItem, error) {
	now := repository.now().UTC()
	result, err := repository.database.ExecContext(ctx, createCarouselStatement,
		item.Position, item.Type, item.Title, item.Description, item.ImageURL, item.VideoURL,
		nullableString(item.VideoSource), nullableString(item.VideoID),
		secondsColumn(item.StartSecond), secondsColumn(item.EndSecond), item.Active, now, now)
	if err != nil {
		return CarouselItem{}, fmt.Errorf("admin: create a carousel item: %w", err)
	}
	itemID, err := result.LastInsertId()
	if err != nil {
		return CarouselItem{}, fmt.Errorf("admin: create a carousel item: %w", err)
	}
	return repository.CarouselItem(ctx, itemID)
}

func (repository *MySQLRepository) UpdateCarouselItem(
	ctx context.Context, itemID int64, edit CarouselEdit,
) (CarouselItem, error) {
	assignments := make([]string, 0, 6)
	arguments := make([]any, 0, 7)
	if edit.Title != nil {
		assignments = append(assignments, "title = ?")
		arguments = append(arguments, *edit.Title)
	}
	if edit.Description != nil {
		assignments = append(assignments, "description = ?")
		arguments = append(arguments, *edit.Description)
	}
	if edit.StartSecond != nil {
		assignments = append(assignments, "video_start_second = ?")
		arguments = append(arguments, secondsColumn(edit.StartSecond))
	}
	if edit.EndSecond != nil {
		assignments = append(assignments, "video_end_second = ?")
		arguments = append(arguments, secondsColumn(edit.EndSecond))
	}
	if edit.Active != nil {
		assignments = append(assignments, "is_active = ?")
		arguments = append(arguments, *edit.Active)
	}
	if len(assignments) == 0 {
		// Nothing to write: answer with the row as it stands rather than touching
		// updated_at for an empty request.
		return repository.CarouselItem(ctx, itemID)
	}

	assignments = append(assignments, "updated_at = ?")
	arguments = append(arguments, repository.now().UTC(), itemID)
	statement := "UPDATE home_carousel_items SET " + strings.Join(assignments, ", ") +
		" WHERE id = ? AND deleted_at IS NULL"

	result, err := repository.database.ExecContext(ctx, statement, arguments...)
	if err != nil {
		return CarouselItem{}, fmt.Errorf("admin: update carousel item %d: %w", itemID, err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		// A write that matched no row means the slide is gone; a write that changed
		// nothing still matches, because updated_at always differs.
		return CarouselItem{}, ErrNotFound
	}
	return repository.CarouselItem(ctx, itemID)
}

const deleteCarouselStatement = `
	UPDATE home_carousel_items SET deleted_at = ?, updated_at = ?
	 WHERE id = ? AND deleted_at IS NULL`

func (repository *MySQLRepository) DeleteCarouselItem(ctx context.Context, itemID int64) error {
	now := repository.now().UTC()
	result, err := repository.database.ExecContext(ctx, deleteCarouselStatement, now, now, itemID)
	if err != nil {
		return fmt.Errorf("admin: delete carousel item %d: %w", itemID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("admin: delete carousel item %d: %w", itemID, err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

const reorderCarouselStatement = `
	UPDATE home_carousel_items SET position = ?, updated_at = ?
	 WHERE id = ? AND deleted_at IS NULL`

// ReorderCarouselItems writes every position in one transaction.
//
// The original updated each row on its own, so a failure halfway left the carousel in an
// order the moderator never asked for and could not see without reloading.
func (repository *MySQLRepository) ReorderCarouselItems(
	ctx context.Context, positions []CarouselPosition,
) error {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("admin: reorder carousel items: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	now := repository.now().UTC()
	for _, entry := range positions {
		result, err := transaction.ExecContext(ctx, reorderCarouselStatement,
			entry.Position, now, entry.ID)
		if err != nil {
			return fmt.Errorf("admin: reorder carousel item %d: %w", entry.ID, err)
		}
		if affected, err := result.RowsAffected(); err == nil && affected == 0 {
			// One missing slide fails the whole reorder: a partial order is not the order
			// that was asked for, and the client's list is stale either way.
			return ErrNotFound
		}
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("admin: reorder carousel items: %w", err)
	}
	return nil
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(destination ...any) error
}

func scanCarouselItem(row rowScanner) (CarouselItem, error) {
	var (
		item        CarouselItem
		startSecond sql.NullString
		endSecond   sql.NullString
	)
	err := row.Scan(&item.ID, &item.Position, &item.Type, &item.Title, &item.Description,
		&item.ImageURL, &item.VideoURL, &item.VideoSource, &item.VideoID,
		&startSecond, &endSecond, &item.Active)
	if errors.Is(err, sql.ErrNoRows) {
		return CarouselItem{}, err
	}
	if err != nil {
		return CarouselItem{}, fmt.Errorf("admin: scan carousel item: %w", err)
	}
	item.StartSecond = secondsValue(startSecond)
	item.EndSecond = secondsValue(endSecond)
	return item, nil
}

// The trim columns are VARCHAR, not integers — that is how the migration created them,
// and 1,000 rows of production data are stored that way. Converted at this boundary so
// nothing above it has to know.
func secondsColumn(value *int) any {
	if value == nil {
		return nil
	}
	return strconv.Itoa(*value)
}

func secondsValue(column sql.NullString) *int {
	if !column.Valid || strings.TrimSpace(column.String) == "" {
		return nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(column.String))
	if err != nil {
		// A column that does not hold a number is treated as unset rather than as an
		// error: the player would ignore it too.
		return nil
	}
	return &value
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// escapeLike makes a keyword match literally.
//
// The original interpolated the keyword straight into a LIKE, so a moderator searching
// for "100%" matched every account and one searching for "_" matched all of them. The
// wildcards are escaped here with a pipe, and the statements declare ESCAPE '|': a
// backslash would mean two different things depending on whether the connection runs with
// NO_BACKSLASH_ESCAPES, and a pipe means one.
func escapeLike(value string) string {
	replacer := strings.NewReplacer(`|`, `||`, `%`, `|%`, `_`, `|_`)
	return replacer.Replace(value)
}
