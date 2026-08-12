package publiccontent

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"2pick.app/backend/internal/postaccess"
)

type MySQLRepository struct {
	database *sql.DB
	now      func() time.Time
	// rankVisibility caches the rank_reports row filter that this database's
	// schema actually supports. See rankVisibilityClause.
	capabilityMu   sync.RWMutex
	rankVisibility *string
}

func NewMySQLRepository(database *sql.DB) *MySQLRepository {
	return &MySQLRepository{database: database, now: time.Now}
}

func (repository *MySQLRepository) Tags(ctx context.Context, keyword string, limit int) ([]Tag, error) {
	arguments := []any{}
	keywordClause := ""
	if keyword != "" {
		keywordClause = " AND t.name LIKE ?"
		arguments = append(arguments, "%"+keyword+"%")
	}
	arguments = append(arguments, limit)
	rows, err := repository.database.QueryContext(ctx, `
		SELECT t.name, COUNT(*)
		FROM tags t
		JOIN post_tags pt ON pt.tag_id = t.id
		JOIN post_policies pp ON pp.post_id = pt.post_id AND pp.access_policy = 'public'
		JOIN posts p ON p.id = pt.post_id AND p.deleted_at IS NULL
		WHERE 1 = 1`+keywordClause+`
		GROUP BY t.id, t.name
		ORDER BY COUNT(*) DESC, t.name ASC
		LIMIT ?`, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]Tag, 0)
	for rows.Next() {
		var tag Tag
		if err := rows.Scan(&tag.Name, &tag.Count); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (repository *MySQLRepository) HotTags(ctx context.Context, postLimit int) (map[string]int64, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT tags
		FROM public_posts
		ORDER BY week_position ASC, id ASC
		LIMIT ?`, postLimit)
	if err != nil {
		return nil, err
	}

	uniqueTags := make([]string, 0)
	seen := make(map[string]struct{})
	for rows.Next() {
		var rawTags string
		if err := rows.Scan(&rawTags); err != nil {
			rows.Close()
			return nil, err
		}
		var tags []string
		if err := json.Unmarshal([]byte(rawTags), &tags); err != nil {
			continue
		}
		for _, tag := range tags {
			if tag == "" {
				continue
			}
			if _, exists := seen[tag]; !exists {
				seen[tag] = struct{}{}
				uniqueTags = append(uniqueTags, tag)
			}
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make(map[string]int64, len(uniqueTags))
	for _, tag := range uniqueTags {
		pattern := "%" + tag + "%"
		var count int64
		if err := repository.database.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM public_posts
			WHERE tags LIKE ? OR title LIKE ? OR description LIKE ?`, pattern, pattern, pattern).Scan(&count); err != nil {
			return nil, err
		}
		result[tag] = count
	}
	return result, nil
}

func (repository *MySQLRepository) CarouselItems(ctx context.Context) ([]CarouselItem, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT title, description, image_url, video_url, position, type,
		       video_source, video_id, video_start_second
		FROM home_carousel_items
		WHERE is_active = 1 AND deleted_at IS NULL
		ORDER BY RAND()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]CarouselItem, 0)
	for rows.Next() {
		var item CarouselItem
		if err := rows.Scan(
			&item.Title, &item.Description, &item.ImageURL, &item.VideoURL,
			&item.Position, &item.Type, &item.VideoSource, &item.VideoID, &item.VideoStartSecond,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *MySQLRepository) Posts(ctx context.Context, query PostsQuery) (PostsPage, error) {
	where, arguments := postFilters(query.Keyword)
	var total int64
	if err := repository.database.QueryRowContext(ctx, "SELECT COUNT(*) FROM public_posts"+where, arguments...).Scan(&total); err != nil {
		return PostsPage{}, err
	}

	sortColumn := postSortColumn(query.Sort, query.Range)
	offset := (query.Page - 1) * query.PerPage
	pageArguments := append(append([]any{}, arguments...), query.PerPage, offset)
	rows, err := repository.database.QueryContext(ctx,
		"SELECT data, description, tags FROM public_posts"+where+" ORDER BY "+sortColumn+" ASC, id ASC LIMIT ? OFFSET ?",
		pageArguments...,
	)
	if err != nil {
		return PostsPage{}, err
	}
	defer rows.Close()

	posts := make([]Post, 0, query.PerPage)
	for rows.Next() {
		var rawData []byte
		var description string
		var rawTags string
		if err := rows.Scan(&rawData, &description, &rawTags); err != nil {
			return PostsPage{}, err
		}
		var post Post
		if err := json.Unmarshal(rawData, &post); err != nil {
			return PostsPage{}, fmt.Errorf("decode public_posts.data: %w", err)
		}
		post.Description = description
		post.Tags = []string{}
		if err := json.Unmarshal([]byte(rawTags), &post.Tags); err != nil {
			post.Tags = []string{}
		}
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return PostsPage{}, err
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(query.PerPage) - 1) / int64(query.PerPage))
	}
	return PostsPage{
		Items: posts, Page: query.Page, PerPage: query.PerPage,
		Total: total, TotalPages: totalPages,
	}, nil
}

func postFilters(keyword string) (string, []any) {
	keywords := strings.Fields(strings.TrimSpace(keyword))
	if len(keywords) > 10 {
		keywords = keywords[:10]
	}
	if len(keywords) == 0 {
		return " WHERE data IS NOT NULL", nil
	}

	clauses := make([]string, 0, len(keywords))
	arguments := make([]any, 0, len(keywords)*3)
	for _, keyword := range keywords {
		keyword = strings.ReplaceAll(keyword, "#", "")
		if keyword == "" {
			continue
		}
		clauses = append(clauses, "(title LIKE ? OR description LIKE ? OR tags LIKE ?)")
		pattern := "%" + keyword + "%"
		arguments = append(arguments, pattern, pattern, pattern)
	}
	if len(clauses) == 0 {
		return " WHERE data IS NOT NULL", nil
	}
	return " WHERE data IS NOT NULL AND " + strings.Join(clauses, " AND "), arguments
}

func postSortColumn(sort, dateRange string) string {
	if sort == "new" {
		return "new_position"
	}
	switch dateRange {
	case "day":
		return "day_position"
	case "month":
		return "month_position"
	default:
		return "week_position"
	}
}

type championRow struct {
	id            int64
	postTitle     string
	postSerial    string
	championID    int64
	championName  string
	championThumb *string
	loserID       sql.NullInt64
	loserName     sql.NullString
	loserThumb    *string
	candidates    sql.NullString
	createdAt     sql.NullTime
}

// championsQuery reads the newest finished games for the home page rail.
//
// JOIN_PREFIX pins user_game_results as the driving table. Without it the
// optimizer starts from posts, scans that table, fans out roughly 190 games per
// post, materialises the whole join into a temp table that spills to disk, and
// only then sorts by ugr.id to keep five rows — 3.2M rows examined and 9-13s on
// production-sized data. Leading with ugr instead lets the primary key satisfy
// ORDER BY ugr.id DESC as a backward index scan, so the rest of the join is
// primary-key lookups that stop as soon as the limit is filled. Measured on a
// 972k-row user_game_results: 8.5s -> 0.11s, byte-identical rows.
//
// The ORDER BY and the leading table belong together: drop either and the plan
// falls back to the temp-table-and-filesort path.
const championsQuery = `
		SELECT /*+ JOIN_PREFIX(ugr) */ ugr.id, p.title, p.serial, ugr.champion_id, ugr.champion_name,
		       COALESCE(NULLIF(champion.lowthumb_url, ''), champion.thumb_url),
		       ugr.loser_id, ugr.loser_name,
		       COALESCE(NULLIF(loser.lowthumb_url, ''), loser.thumb_url),
		       ugr.candidates, ugr.created_at
		FROM user_game_results ugr
		JOIN games g ON g.id = ugr.game_id
		JOIN posts p ON p.id = g.post_id AND p.deleted_at IS NULL
		JOIN post_policies pp ON pp.post_id = p.id AND pp.access_policy = 'public'
		JOIN elements champion ON champion.id = ugr.champion_id AND champion.deleted_at IS NULL
		LEFT JOIN elements loser ON loser.id = ugr.loser_id AND loser.deleted_at IS NULL
		WHERE p.is_censored = 0
		ORDER BY ugr.id DESC
		LIMIT ?`

func (repository *MySQLRepository) Champions(ctx context.Context, limit int) ([]Champion, error) {
	rows, err := repository.database.QueryContext(ctx, championsQuery, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	champions := make([]Champion, 0, limit)
	for rows.Next() {
		var row championRow
		if err := rows.Scan(
			&row.id, &row.postTitle, &row.postSerial, &row.championID, &row.championName,
			&row.championThumb, &row.loserID, &row.loserName, &row.loserThumb,
			&row.candidates, &row.createdAt,
		); err != nil {
			return nil, err
		}
		champions = append(champions, buildChampion(row))
	}
	return champions, rows.Err()
}

func buildChampion(row championRow) Champion {
	championElement := &ChampionElement{Name: row.championName, ThumbURL: row.championThumb, IsWinner: true}
	var loserElement *ChampionElement
	if row.loserID.Valid {
		loserElement = &ChampionElement{Name: row.loserName.String, ThumbURL: row.loserThumb, IsWinner: false}
	}

	left, right := championElement, loserElement
	if row.candidates.Valid {
		parts := strings.Split(row.candidates.String, ",")
		if len(parts) >= 2 {
			left = candidateElement(parts[0], row.championID, row.loserID, championElement, loserElement)
			right = candidateElement(parts[1], row.championID, row.loserID, championElement, loserElement)
		}
	}
	dateTime := ""
	if row.createdAt.Valid {
		dateTime = row.createdAt.Time.UTC().Format(time.RFC3339)
	}
	key := fmt.Sprintf("%x", md5.Sum([]byte(strconv.FormatInt(row.id, 10)))) // Matches the legacy public response key; not used for security.
	return Champion{
		PostTitle: row.postTitle, PostSerial: row.postSerial, Left: left, Right: right,
		DateTime: dateTime, ThumbURL: row.championThumb, Key: key,
	}
}

func candidateElement(value string, championID int64, loserID sql.NullInt64, champion, loser *ChampionElement) *ChampionElement {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return nil
	}
	if id == championID {
		return champion
	}
	if loserID.Valid && id == loserID.Int64 {
		return loser
	}
	return nil
}

func (repository *MySQLRepository) Ranks(ctx context.Context, postSerial string, group RankGroup, page, perPage int, caller postaccess.Caller) (RanksPage, error) {
	postID, err := repository.visiblePostID(ctx, postSerial, caller)
	if err != nil {
		return RanksPage{}, err
	}
	visibilityClause, err := repository.rankVisibilityClause(ctx)
	if err != nil {
		return RanksPage{}, err
	}
	if group == RankGroupRecent1000 {
		return repository.recentRanks(ctx, postID, visibilityClause, page, perPage)
	}
	return repository.cumulativeRanks(ctx, postID, visibilityClause, page, perPage)
}

func (repository *MySQLRepository) cumulativeRanks(ctx context.Context, postID int64, visibilityClause string, page, perPage int) (RanksPage, error) {
	var total int64
	if err := repository.database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM rank_reports rr
		JOIN elements e ON e.id = rr.element_id
		WHERE rr.post_id = ? AND e.deleted_at IS NULL`+visibilityClause,
		postID,
	).Scan(&total); err != nil {
		return RanksPage{}, err
	}

	offset := (page - 1) * perPage
	rows, err := repository.database.QueryContext(ctx, rankReportSelect+`
		WHERE rr.post_id = ? AND e.deleted_at IS NULL`+visibilityClause+`
		ORDER BY rr.rank ASC, rr.id ASC
		LIMIT ? OFFSET ?`, postID, perPage, offset)
	if err != nil {
		return RanksPage{}, err
	}
	defer rows.Close()

	reports := make([]RankReport, 0, perPage)
	for rows.Next() {
		report, err := scanRankReport(rows, repository.now())
		if err != nil {
			return RanksPage{}, err
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return RanksPage{}, err
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(perPage) - 1) / int64(perPage))
	}
	return RanksPage{
		Items: reports, Group: RankGroupCumulative, Page: page, PerPage: perPage, Total: total, TotalPages: totalPages,
	}, nil
}

func (repository *MySQLRepository) recentRanks(ctx context.Context, postID int64, visibilityClause string, page, perPage int) (RanksPage, error) {
	const latestSnapshot = `
		SELECT MAX(latest.start_date)
		FROM rank_report_histories latest
		WHERE latest.post_id = ? AND latest.time_range = 'thousand_votes'
		  AND latest.rank > 0 AND latest.deleted_at IS NULL`
	filters := `
		WHERE rrh.post_id = ? AND rrh.time_range = 'thousand_votes'
		  AND rrh.start_date = (` + latestSnapshot + `)
		  AND rrh.rank > 0 AND rrh.deleted_at IS NULL
		  AND e.deleted_at IS NULL` + visibilityClause

	var total int64
	if err := repository.database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM rank_report_histories rrh
		JOIN rank_reports rr ON rr.id = rrh.rank_report_id
		JOIN elements e ON e.id = rrh.element_id
		`+filters, postID, postID).Scan(&total); err != nil {
		return RanksPage{}, err
	}

	offset := (page - 1) * perPage
	rows, err := repository.database.QueryContext(ctx, recentRankReportSelect+filters+`
		ORDER BY rrh.rank ASC, rrh.id ASC
		LIMIT ? OFFSET ?`, postID, postID, perPage, offset)
	if err != nil {
		return RanksPage{}, err
	}
	defer rows.Close()

	reports := make([]RankReport, 0, perPage)
	for rows.Next() {
		report, err := scanHistoricalRankReport(rows)
		if err != nil {
			return RanksPage{}, err
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return RanksPage{}, err
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(perPage) - 1) / int64(perPage))
	}
	return RanksPage{
		Items: reports, Group: RankGroupRecent1000, Page: page, PerPage: perPage, Total: total, TotalPages: totalPages,
	}, nil
}

func (repository *MySQLRepository) SearchRanks(ctx context.Context, postSerial, keyword string, limit int, caller postaccess.Caller) ([]RankReport, error) {
	postID, err := repository.visiblePostID(ctx, postSerial, caller)
	if err != nil {
		return nil, err
	}
	visibilityClause, err := repository.rankVisibilityClause(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := repository.database.QueryContext(ctx, rankReportSelect+`
		WHERE rr.post_id = ? AND e.title LIKE ?
		  AND e.deleted_at IS NULL`+visibilityClause+`
		ORDER BY rr.rank ASC
		LIMIT ?`, postID, "%"+keyword+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := make([]RankReport, 0)
	for rows.Next() {
		report, err := scanRankReport(rows, repository.now())
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, rows.Err()
}

func (repository *MySQLRepository) Rank(ctx context.Context, postSerial string, elementID int64, ranges []string, caller postaccess.Caller) (RankDetails, error) {
	postID, err := repository.visiblePostID(ctx, postSerial, caller)
	if err != nil {
		return RankDetails{}, err
	}
	if err := repository.requireElement(ctx, elementID); err != nil {
		return RankDetails{}, err
	}
	visibilityClause, err := repository.rankVisibilityClause(ctx)
	if err != nil {
		return RankDetails{}, err
	}
	row := repository.database.QueryRowContext(ctx, rankReportSelect+`
		WHERE rr.post_id = ? AND rr.element_id = ?
		  AND e.deleted_at IS NULL`+visibilityClause+`
		LIMIT 1`, postID, elementID)
	report, err := scanRankReport(row, repository.now())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return RankDetails{}, err
	}

	details := RankDetails{History: make(map[string][]RankHistory, len(ranges))}
	if err == nil {
		details.Current = &report
		details.Groups.Cumulative = &report
	}
	for _, dateRange := range ranges {
		history, err := repository.rankHistory(ctx, postID, elementID, dateRange, historyLimit(dateRange))
		if err != nil {
			return RankDetails{}, err
		}
		details.History[dateRange] = history
		if dateRange == "thousand_votes" {
			details.Groups.Recent1000 = recentGroupReport(details.Current, history)
		}
	}
	return details, nil
}

// recentGroupReport picks the most recent snapshot in which the element was
// actually ranked.
//
// rank == 0 is rank_report_histories' marker for "present in this snapshot but
// not ranked", and the newest snapshot very often carries it: on one 200-element
// post, 186 elements had a zero-rank newest row. Reading history[0] blindly
// therefore reported "no data" for almost every element, while the list endpoint
// showed a real rank — recentRanks selects the latest snapshot among rows with
// rrh.rank > 0, and this has to agree with it. History arrives newest-first, so
// the first positive entry is the newest positive one.
func recentGroupReport(current *RankReport, history []RankHistory) *RankReport {
	if current == nil {
		return nil
	}
	for _, entry := range history {
		if entry.Rank <= 0 {
			continue
		}
		rank := entry.Rank
		return &RankReport{
			Rank: &rank, WinRate: entry.WinRate, Date: entry.Date, Element: current.Element,
		}
	}
	return nil
}

func (repository *MySQLRepository) requireElement(ctx context.Context, elementID int64) error {
	var exists int
	err := repository.database.QueryRowContext(ctx, `
		SELECT 1 FROM elements WHERE id = ? AND deleted_at IS NULL LIMIT 1`, elementID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

const rankReportSelect = `
	SELECT rr.rank, rr.win_rate,
	       e.title, e.type, e.id, e.video_id, e.source_url, e.video_source,
	       e.thumb_url,
	       e.lowthumb_url, e.mediumthumb_url
	FROM rank_reports rr
	JOIN elements e ON e.id = rr.element_id
`

const recentRankReportSelect = `
	SELECT rrh.rank, rrh.win_rate, rrh.start_date,
	       e.title, e.type, e.id, e.video_id, e.source_url, e.video_source,
	       e.thumb_url,
	       e.lowthumb_url, e.mediumthumb_url
	FROM rank_report_histories rrh
	JOIN rank_reports rr ON rr.id = rrh.rank_report_id
	JOIN elements e ON e.id = rrh.element_id
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRankReport(scanner rowScanner, now time.Time) (RankReport, error) {
	var rank sql.NullInt64
	var winRate sql.NullFloat64
	var element RankElement
	if err := scanner.Scan(
		&rank, &winRate, &element.Title, &element.Type, &element.ID,
		&element.VideoID, &element.SourceURL, &element.VideoSource, &element.ThumbURL,
		&element.LowThumbURL, &element.MediumThumbURL,
	); err != nil {
		return RankReport{}, err
	}
	var rankValue *int64
	if rank.Valid {
		rankValue = &rank.Int64
	}
	return RankReport{
		Rank: rankValue, WinRate: formatRate(winRate), Date: now.Format("2006-01-02"), Element: element,
	}, nil
}

func scanHistoricalRankReport(scanner rowScanner) (RankReport, error) {
	var rank sql.NullInt64
	var winRate sql.NullFloat64
	var recordDate time.Time
	var element RankElement
	if err := scanner.Scan(
		&rank, &winRate, &recordDate, &element.Title, &element.Type, &element.ID,
		&element.VideoID, &element.SourceURL, &element.VideoSource, &element.ThumbURL,
		&element.LowThumbURL, &element.MediumThumbURL,
	); err != nil {
		return RankReport{}, err
	}
	var rankValue *int64
	if rank.Valid {
		rankValue = &rank.Int64
	}
	return RankReport{
		Rank: rankValue, WinRate: formatRate(winRate), Date: recordDate.Format("2006-01-02"), Element: element,
	}, nil
}

func (repository *MySQLRepository) rankHistory(ctx context.Context, postID, elementID int64, dateRange string, limit int) ([]RankHistory, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT rrh.rank, rrh.win_rate, rrh.start_date
		FROM rank_report_histories rrh
		WHERE rrh.post_id = ? AND rrh.element_id = ? AND rrh.time_range = ? AND rrh.deleted_at IS NULL
		ORDER BY rrh.start_date DESC
		LIMIT ?`, postID, elementID, dateRange, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	history := make([]RankHistory, 0)
	for rows.Next() {
		var item RankHistory
		var winRate sql.NullFloat64
		var startDate time.Time
		if err := rows.Scan(&item.Rank, &winRate, &startDate); err != nil {
			return nil, err
		}
		item.WinRate = formatRate(winRate)
		item.Date = startDate.Format("2006-01-02")
		history = append(history, item)
	}
	return history, rows.Err()
}

func formatRate(rate sql.NullFloat64) string {
	if !rate.Valid {
		return "0"
	}
	return strconv.FormatFloat(math.Round(rate.Float64*10)/10, 'f', 1, 64)
}

func historyLimit(dateRange string) int {
	switch dateRange {
	case "week":
		return 12
	case "month":
		return 3
	case "year":
		return 10
	default:
		return 90
	}
}

// visiblePostID resolves a serial to a post id, or ErrNotFound if this caller may not see
// it. Not found rather than forbidden on purpose: a stranger learns nothing about which
// serials exist.
func (repository *MySQLRepository) visiblePostID(
	ctx context.Context, serial string, caller postaccess.Caller,
) (int64, error) {
	visible, visibleArguments := postaccess.VisibilityClause("p", "pp", caller)
	var postID int64
	err := repository.database.QueryRowContext(ctx, `
		SELECT p.id
		FROM posts p
		JOIN post_policies pp ON pp.post_id = p.id
		WHERE p.serial = ? AND p.deleted_at IS NULL AND `+visible+`
		LIMIT 1`, append([]any{serial}, visibleArguments...)...).Scan(&postID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return postID, err
}

// rankVisibilityClause builds the rank_reports row filter for whichever of the
// soft-delete and moderation columns this database actually has.
//
// The column pair is not stable across schema snapshots: a database restored
// from one era has rank_reports.deleted_at and no hidden, another has hidden and
// no deleted_at. Hard-coding either one takes the whole ranking endpoint down
// with "Unknown column" on the other. Probing both together also replaces the
// previous hidden-only probe, so this still costs one information_schema query
// per process, cached behind capabilityMu.
func (repository *MySQLRepository) rankVisibilityClause(ctx context.Context) (string, error) {
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

	clause := rankVisibility(hasDeletedAt, hasHidden)
	repository.capabilityMu.Lock()
	repository.rankVisibility = &clause
	repository.capabilityMu.Unlock()
	return clause, nil
}

// rankVisibility assembles the filter in a fixed order, so the generated SQL is
// stable no matter which order information_schema returns the columns in. A
// column this database does not have contributes no filter rather than an error.
func rankVisibility(hasDeletedAt, hasHidden bool) string {
	clause := ""
	if hasDeletedAt {
		clause += " AND rr.deleted_at IS NULL"
	}
	if hasHidden {
		clause += " AND rr.hidden = 0"
	}
	return clause
}
