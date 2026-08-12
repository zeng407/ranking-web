package posttrend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"2pick.app/backend/internal/jobs"
	"2pick.app/backend/internal/queue"
)

// Handler contracts.
//
// The create pass scans the games table — 959k rows, 757k of them qualifying — in one
// GROUP BY and then writes around 6,200 statistics rows, so it is the slower of the
// two. Both stay under the worker's 90s job timeout.
const (
	CreateTimeout    = 60 * time.Second
	PositionsTimeout = 60 * time.Second
	CreateAttempts   = 3
	PositionAttempts = 3
)

// LockPrefix serializes work on the same range. Two runs for one range would both
// reset positions to the sentinel and interleave their upserts, briefly leaving the
// home page ranked by nothing.
const LockPrefix = "posttrend:"

// CreatePayload carries the schedule's range argument.
type CreatePayload struct {
	// Range is the artisan argument, so "day" rather than "today". Translated by
	// RangeFromScheduleArgument.
	Range string `json:"range"`
}

// PositionsPayload names the group to rank. The range here is already the stored
// enum value, because the create handler resolved it.
type PositionsPayload struct {
	Range TimeRange `json:"range"`
	// StartDate is the window start as YYYY-MM-DD, empty for the all-time range.
	// A string rather than a *time.Time so the wire form is unambiguous and a
	// timezone cannot ride along with what is a DATE.
	StartDate string `json:"start_date,omitempty"`
}

// Options wires the service.
type Options struct {
	Repository Repository
	Publisher  *queue.Publisher
	Logger     *slog.Logger
	// Location is the timezone the DATE windows are computed in. These are calendar
	// dates and Laravel's today() uses the application timezone, so this must not
	// fall back to the container's UTC.
	Location *time.Location
	// Now is overridable so a test can pin the window.
	Now func() time.Time
}

// Service owns the two handlers.
type Service struct {
	repository Repository
	publisher  *queue.Publisher
	logger     *slog.Logger
	location   *time.Location
	now        func() time.Time
}

func NewService(options Options) (*Service, error) {
	if options.Repository == nil {
		return nil, errors.New("posttrend: repository is required")
	}
	if options.Publisher == nil {
		return nil, errors.New("posttrend: publisher is required")
	}
	if options.Location == nil {
		return nil, errors.New("posttrend: location is required")
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		repository: options.Repository,
		publisher:  options.Publisher,
		logger:     logger,
		location:   options.Location,
		now:        now,
	}, nil
}

// CreateRegistration replaces the make:post-trend command.
func (service *Service) CreateRegistration() jobs.Registration {
	return jobs.Registration{
		Type:        TypeCreate,
		Handler:     jobs.HandlerFunc(service.handleCreate),
		Timeout:     CreateTimeout,
		MaxAttempts: CreateAttempts,
		SerialKey:   service.createSerialKey,
		LaravelJob:  "App\\Console\\Commands\\CreatePostTrend",
	}
}

// PositionsRegistration replaces UpdatePostTrendsPosition.
func (service *Service) PositionsRegistration() jobs.Registration {
	return jobs.Registration{
		Type:        TypeUpdatePositions,
		Handler:     jobs.HandlerFunc(service.handlePositions),
		Timeout:     PositionsTimeout,
		MaxAttempts: PositionAttempts,
		SerialKey:   service.positionsSerialKey,
		LaravelJob:  "App\\Jobs\\UpdatePostTrendsPosition",
	}
}

func (service *Service) createSerialKey(message queue.Message) (string, error) {
	var payload CreatePayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return "", fmt.Errorf("posttrend: decode payload for serial key: %w", err)
	}
	rangeValue, err := RangeFromScheduleArgument(payload.Range)
	if err != nil {
		return "", err
	}
	// Keyed on the resolved range so "day" and "today" cannot both run at once.
	return LockPrefix + "create:" + string(rangeValue), nil
}

func (service *Service) positionsSerialKey(message queue.Message) (string, error) {
	var payload PositionsPayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return "", fmt.Errorf("posttrend: decode payload for serial key: %w", err)
	}
	if !payload.Range.Valid() {
		return "", fmt.Errorf("%w: %q", ErrUnknownRange, payload.Range)
	}
	// Matches uniqueId() in the Laravel job, which was startDate . range.
	return LockPrefix + "positions:" + string(payload.Range) + ":" + payload.StartDate, nil
}

// handleCreate recomputes play counts for one range, then asks for the positions to
// be reassigned.
func (service *Service) handleCreate(ctx context.Context, message queue.Message) error {
	var payload CreatePayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return jobs.Permanent(fmt.Errorf("posttrend: decode create payload: %w", err))
	}
	rangeValue, err := RangeFromScheduleArgument(payload.Range)
	if err != nil {
		// A bad range will stay bad, so retrying cannot help.
		return jobs.Permanent(err)
	}

	windowStart := WindowStart(rangeValue, service.now().In(service.location))
	started := time.Now()

	counts, err := service.repository.PlayCounts(ctx, rangeValue, windowStart)
	if err != nil {
		return err
	}
	written, err := service.repository.UpsertPlayCounts(ctx, rangeValue, counts)
	if err != nil {
		return err
	}

	// Published after the statistics are written, not before: the positions pass
	// reads exactly these rows, and running it first would rank the previous hour's
	// counts and then leave them stale until the next tick.
	positions, err := PositionsMessage(rangeValue, windowStart)
	if err != nil {
		return err
	}
	if err := service.publisher.Publish(ctx, positions); err != nil {
		return err
	}

	service.logger.Info("post_trend_created",
		"range", string(rangeValue),
		"window_start", formatWindow(windowStart),
		"posts", len(counts),
		"rows_written", written,
		"duration_ms", time.Since(started).Milliseconds(),
	)
	return nil
}

// handlePositions turns play counts into ranked positions.
func (service *Service) handlePositions(ctx context.Context, message queue.Message) error {
	var payload PositionsPayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return jobs.Permanent(fmt.Errorf("posttrend: decode positions payload: %w", err))
	}
	if !payload.Range.Valid() {
		return jobs.Permanent(fmt.Errorf("%w: %q", ErrUnknownRange, payload.Range))
	}

	windowStart, err := parseWindow(payload.Range, payload.StartDate)
	if err != nil {
		return jobs.Permanent(err)
	}
	started := time.Now()

	// Reset first. A post that has dropped out of the top RankedLimit gets no upsert,
	// so without this it would keep the position it held last hour and outrank posts
	// that are actually being played.
	reset, err := service.repository.ResetPositions(ctx, payload.Range, windowStart)
	if err != nil {
		return err
	}

	postIDs, err := service.repository.RankedPosts(ctx, payload.Range, windowStart, RankedLimit)
	if err != nil {
		return err
	}

	positions := make([]TrendPosition, 0, len(postIDs))
	for index, postID := range postIDs {
		positions = append(positions, TrendPosition{PostID: postID, Position: index + 1})
	}
	written, err := service.repository.UpsertPositions(ctx, payload.Range, windowStart, positions)
	if err != nil {
		return err
	}

	service.logger.Info("post_trend_positions_updated",
		"range", string(payload.Range),
		"window_start", formatWindow(windowStart),
		"reset_rows", reset,
		"ranked", len(positions),
		"rows_written", written,
		"duration_ms", time.Since(started).Milliseconds(),
	)
	return nil
}

// PositionsMessage builds the follow-up message. Exported so the ranking pass can be
// triggered on its own during a backfill.
func PositionsMessage(rangeValue TimeRange, windowStart *time.Time) (queue.Message, error) {
	if !rangeValue.Valid() {
		return queue.Message{}, fmt.Errorf("%w: %q", ErrUnknownRange, rangeValue)
	}
	payload := PositionsPayload{Range: rangeValue}
	if windowStart != nil {
		payload.StartDate = windowStart.Format(dateLayout)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return queue.Message{}, fmt.Errorf("posttrend: encode positions payload: %w", err)
	}
	return queue.Message{
		Queue:   Queue,
		Type:    TypeUpdatePositions,
		Payload: body,
		// One group is ranked once per tick. A redelivery redoes the same reset and
		// upserts, which is idempotent.
		IdempotencyKey: string(rangeValue) + ":" + payload.StartDate,
	}, nil
}

// parseWindow turns the wire date back into a window, enforcing the invariant that
// only the all-time range has no start date.
func parseWindow(rangeValue TimeRange, value string) (*time.Time, error) {
	if rangeValue == RangeAll {
		if value != "" {
			return nil, fmt.Errorf("posttrend: the %q range must carry no start date, got %q", rangeValue, value)
		}
		return nil, nil
	}
	if value == "" {
		return nil, fmt.Errorf("posttrend: the %q range needs a start date", rangeValue)
	}
	// Parsed as a bare date in UTC: it is only ever formatted back to YYYY-MM-DD for
	// a DATE column, so no timezone is attached to it.
	parsed, err := time.Parse(dateLayout, value)
	if err != nil {
		return nil, fmt.Errorf("posttrend: start date %q: %w", value, err)
	}
	return &parsed, nil
}

func formatWindow(windowStart *time.Time) string {
	if windowStart == nil {
		return "all"
	}
	return windowStart.Format(dateLayout)
}
