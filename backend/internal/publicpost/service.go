package publicpost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"2pick.app/backend/internal/jobs"
	"2pick.app/backend/internal/queue"
)

// RefreshTimeout bounds one refresh.
//
// Generous on purpose. Removing the 2,000-row cap means a pass now covers every
// qualifying post, so the run is longer than the PHP's by design; the Kernel entry
// carries withoutOverlapping(60) and the scheduler entry the same TTL, so a long run
// is expected rather than something to cut short. This still has to stay under the
// worker's job timeout, which is capped by Laravel's redis retry_after of 90s.
const RefreshTimeout = 85 * time.Second

// RefreshAttempts bounds redelivery. A refresh is idempotent — it recomputes every
// position from scratch — so retrying is safe.
const RefreshAttempts = 3

// Options wires the service.
type Options struct {
	Repository Repository
	Freshness  FreshnessStore
	Cache      ResourceCache
	Logger     *slog.Logger
	// Location is the timezone the trend windows are computed in. The windows are DATE
	// values and Laravel's today() uses the application timezone.
	Location *time.Location
	Now      func() time.Time
	// Shuffle permutes the preview candidates. Injected so a test can pin the choice;
	// the preview is deliberately random, see SelectPreviewElements.
	Shuffle func(length int, swap func(i, j int))
}

// Service owns the refresh handler.
type Service struct {
	repository Repository
	freshness  FreshnessStore
	cache      ResourceCache
	logger     *slog.Logger
	location   *time.Location
	now        func() time.Time
	shuffle    func(length int, swap func(i, j int))
}

func NewService(options Options) (*Service, error) {
	if options.Repository == nil {
		return nil, errors.New("publicpost: repository is required")
	}
	if options.Freshness == nil {
		return nil, errors.New("publicpost: freshness store is required")
	}
	if options.Location == nil {
		return nil, errors.New("publicpost: location is required")
	}
	cache := options.Cache
	if cache == nil {
		cache = NoResourceCache{}
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	shuffle := options.Shuffle
	if shuffle == nil {
		shuffle = rand.Shuffle
	}
	return &Service{
		repository: options.Repository,
		freshness:  options.Freshness,
		cache:      cache,
		logger:     logger,
		location:   options.Location,
		now:        now,
		shuffle:    shuffle,
	}, nil
}

// Registration replaces PublicPostScheduleExecutor::updatePublicPosts.
func (service *Service) Registration() jobs.Registration {
	return jobs.Registration{
		Type:        TypeRefresh,
		Handler:     jobs.HandlerFunc(service.handleRefresh),
		Timeout:     RefreshTimeout,
		MaxAttempts: RefreshAttempts,
		// One lock for the whole table: the passes share the is_dirty column, so two
		// refreshes interleaving would clear each other's flags and blank listings.
		SerialKey:  func(queue.Message) (string, error) { return LockKey, nil },
		LaravelJob: "App\\ScheduleExecutor\\PublicPostScheduleExecutor",
	}
}

// handleRefresh rebuilds every listing position.
func (service *Service) handleRefresh(ctx context.Context, message queue.Message) error {
	if len(message.Payload) > 0 && !json.Valid(message.Payload) {
		return jobs.Permanent(errors.New("publicpost: refresh payload is not valid JSON"))
	}

	fresh, err := service.freshness.IsFresh(ctx)
	if err != nil {
		return err
	}
	if fresh {
		// The debounce. PostUpdateTimestampSubscriber clears the flag whenever a post
		// changes, so a change is picked up on the next tick and an idle system rebuilds
		// at most once per FreshnessTTL.
		service.logger.Debug("public_post_refresh_skipped", "reason", "still fresh")
		return nil
	}

	started := time.Now()
	today := service.now().In(service.location)

	// Every pass runs even if an earlier one failed, matching the PHP's four separate
	// try/catch blocks: a broken week trend must not stop the month listing from being
	// rebuilt. The errors are collected so the job still reports failure and retries.
	var failures []error
	completed := 0
	for _, pass := range Ordered() {
		written, err := service.runPass(ctx, pass, today)
		if err != nil {
			service.logger.Error("public_post_pass_failed", "pass", string(pass), "error", err)
			failures = append(failures, fmt.Errorf("%s pass: %w", pass, err))
			continue
		}
		completed++
		service.logger.Info("public_post_pass_finished", "pass", string(pass), "written", written)
	}

	removed, err := service.repository.RemoveDirty(ctx)
	if err != nil {
		failures = append(failures, fmt.Errorf("remove stale listings: %w", err))
	}

	if len(failures) > 0 {
		// Not marked fresh: the next tick should try again rather than wait out the
		// debounce on a half-built listing.
		return errors.Join(failures...)
	}

	// Only after every pass and the cleanup succeeded, matching where the PHP set it.
	if err := service.freshness.MarkFresh(ctx); err != nil {
		return err
	}

	// Best effort. A stale Laravel resource cache is a nuisance; failing the job over
	// it would repeat the whole rebuild.
	postIDs, err := service.repository.PublicPostIDs(ctx)
	if err != nil {
		service.logger.Warn("public_post_resource_cache_not_cleared", "error", err)
	} else if err := service.cache.Clear(ctx, postIDs); err != nil {
		service.logger.Warn("public_post_resource_cache_not_cleared",
			"posts", len(postIDs), "error", err)
	}

	service.logger.Info("public_post_refresh_finished",
		"passes", completed,
		"removed", removed,
		"listed", len(postIDs),
		"duration_ms", time.Since(started).Milliseconds(),
	)
	return nil
}

// runPass rebuilds one position column.
func (service *Service) runPass(ctx context.Context, pass Pass, today time.Time) (int, error) {
	postIDs, err := service.sourceIDs(ctx, pass, today)
	if err != nil {
		return 0, err
	}

	// The trend passes returned early on an empty set before touching anything, and
	// that guard is load-bearing: marking every row dirty and then writing nothing
	// would push the whole listing to the sentinel and empty the page. PassNew has no
	// such guard in the original, but an empty result there means no post qualifies at
	// all, so the same protection applies.
	if len(postIDs) == 0 {
		service.logger.Warn("public_post_pass_skipped",
			"pass", string(pass), "reason", "no qualifying posts")
		return 0, nil
	}

	if _, err := service.repository.MarkAllDirty(ctx); err != nil {
		return 0, err
	}

	written := 0
	for start := 0; start < len(postIDs); start += ChunkSize {
		end := start + ChunkSize
		if end > len(postIDs) {
			end = len(postIDs)
		}
		chunk := postIDs[start:end]

		rows, err := service.repository.LoadChunk(ctx, chunk)
		if err != nil {
			return written, err
		}

		// Positions follow the source order, which is the position within the whole
		// pass rather than within the chunk. The PHP incremented one counter across the
		// entire loop, so a chunk boundary must not restart it.
		position := start
		byPost := make(map[int64]int, len(chunk))
		for _, postID := range chunk {
			position++
			byPost[postID] = position
		}

		for index := range rows {
			service.finalise(&rows[index], byPost[rows[index].PostID])
		}

		count, err := service.repository.UpsertChunk(ctx, pass, rows)
		if err != nil {
			return written, err
		}
		_ = count
		written += len(rows)
	}

	if _, err := service.repository.PushDirtyToSentinel(ctx, pass); err != nil {
		return written, err
	}
	return written, nil
}

// finalise picks the preview elements and encodes the tag column.
func (service *Service) finalise(row *Row, position int) {
	row.Position = position

	element1, element2 := SelectPreviewElements(row.rankedCandidates, row.fallbackCandidates, service.shuffle)
	row.Resource.Element1 = BuildElement(element1)
	row.Resource.Element2 = BuildElement(element2)

	// The tags column holds the same list as the payload, JSON encoded. Laravel wrote
	// it with JSON_UNESCAPED_UNICODE; Go's encoder escapes < > & by default but not
	// non-ASCII, so the CJK tag names come out unescaped either way. HTML escaping is
	// turned off below so a tag containing an angle bracket matches too.
	row.Tags = encodeTags(row.tagNames)
	row.Resource.Tags = row.tagNames
	if row.Resource.Tags == nil {
		row.Resource.Tags = []string{}
	}
}

// sourceIDs is the ordered set of posts a pass covers.
func (service *Service) sourceIDs(ctx context.Context, pass Pass, today time.Time) ([]int64, error) {
	if pass == PassNew {
		return service.repository.ListedPostIDs(ctx)
	}
	windowStart, err := TrendWindowStart(pass, today)
	if err != nil {
		return nil, err
	}
	return service.repository.TrendedPostIDs(ctx, pass.TrendRange(), windowStart)
}

// TrendWindowStart is the window a trend pass reads, matching the executor's
// today()/startOfWeek()/startOfMonth() calls. The week starts on Monday, Carbon's ISO
// default; see posttrend.WindowStart for the evidence.
func TrendWindowStart(pass Pass, now time.Time) (time.Time, error) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch pass {
	case PassToday:
		return today, nil
	case PassWeek:
		// Sunday is weekday 0 in Go but the last day of an ISO week, so it steps back
		// six days rather than none.
		offset := (int(today.Weekday()) + 6) % 7
		return today.AddDate(0, 0, -offset), nil
	case PassMonth:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), nil
	}
	return time.Time{}, fmt.Errorf("%w: %q has no trend window", ErrUnknownPass, pass)
}

// encodeTags produces the JSON array stored in the tags column.
func encodeTags(names []string) string {
	if names == nil {
		names = []string{}
	}
	buffer := &jsonBuffer{}
	encoder := json.NewEncoder(buffer)
	// Matches PHP's JSON_UNESCAPED_UNICODE more closely: without this Go would turn a
	// tag containing < or & into <, which the stored column never had.
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(names); err != nil {
		// Encoding a []string cannot fail; an empty array is still valid JSON if it
		// somehow did.
		return "[]"
	}
	return buffer.trimmed()
}

// jsonBuffer collects the encoder output. json.Encoder appends a newline, which the
// column must not carry.
type jsonBuffer struct {
	data []byte
}

func (buffer *jsonBuffer) Write(chunk []byte) (int, error) {
	buffer.data = append(buffer.data, chunk...)
	return len(chunk), nil
}

func (buffer *jsonBuffer) trimmed() string {
	for len(buffer.data) > 0 && (buffer.data[len(buffer.data)-1] == '\n' || buffer.data[len(buffer.data)-1] == '\r') {
		buffer.data = buffer.data[:len(buffer.data)-1]
	}
	return string(buffer.data)
}

// RefreshMessage builds the message the schedule enqueues.
func RefreshMessage() (queue.Message, error) {
	body, err := json.Marshal(struct{}{})
	if err != nil {
		return queue.Message{}, fmt.Errorf("publicpost: encode refresh payload: %w", err)
	}
	return queue.Message{Queue: Queue, Type: TypeRefresh, Payload: body}, nil
}
