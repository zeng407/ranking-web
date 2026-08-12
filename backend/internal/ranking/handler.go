package ranking

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"2pick.app/backend/internal/jobs"
	"2pick.app/backend/internal/queue"
)

// Queue message types handled by this package.
const (
	// MessageTypeUpdateElementRank replaces App\Jobs\UpdateElementRank.
	MessageTypeUpdateElementRank = "rank.update_element"
	// MessageTypeUpdateRankReport replaces App\Jobs\UpdateRankReport.
	MessageTypeUpdateRankReport = "rank.update_report"
	// MessageTypeCreateRankHistory replaces App\Jobs\CreateRankReportHistory,
	// which builds the `all` and `thousand_votes` ranges for one report.
	MessageTypeCreateRankHistory = "rank.create_history"
)

// QueueRankReportHistory matches the queue CreateRankReportHistory declares with
// onQueue('rank_report_history'). It is separate so the long history builds cannot
// starve the default queue.
const QueueRankReportHistory = "rank_report_history"

// Queue names match the ones the Laravel jobs already declare.
const (
	// QueueDefault carries element rank updates, as UpdateElementRank does today.
	QueueDefault = "default"
)

// Handler contracts.
//
// Laravel's UpdateElementRank declares none of these; it inherits whatever the
// worker was started with and relies on ShouldBeUnique for deduplication.
const (
	// UpdateElementRankTimeout bounds one attempt. The four aggregation queries
	// hit game_1v1_rounds, which holds 50.7M rows, so this is generous, but it
	// stays under the worker's 90s ceiling (Laravel's redis retry_after) so a job
	// cannot outlive the window in which redelivery is assumed not to happen.
	UpdateElementRankTimeout = 60 * time.Second
	// UpdateElementRankMaxAttempts bounds redelivery. The work is convergent, so
	// a retry is always safe.
	UpdateElementRankMaxAttempts = 5
)

// UpdateElementRankPayload is the message body.
type UpdateElementRankPayload struct {
	PostID    int64 `json:"post_id"`
	ElementID int64 `json:"element_id"`
}

// NewUpdateElementRankMessage builds a dispatchable message.
//
// The idempotency key is the natural key of the work rather than a timestamp:
// two dispatches for the same element are the same work, and the computation is
// convergent, so a redelivery is detectable without being harmful.
func NewUpdateElementRankMessage(postID, elementID int64) (queue.Message, error) {
	if postID <= 0 || elementID <= 0 {
		return queue.Message{}, fmt.Errorf("ranking: post id and element id are required, got post=%d element=%d", postID, elementID)
	}
	payload, err := json.Marshal(UpdateElementRankPayload{PostID: postID, ElementID: elementID})
	if err != nil {
		return queue.Message{}, fmt.Errorf("ranking: encode payload: %w", err)
	}
	return queue.Message{
		Queue:          QueueDefault,
		Type:           MessageTypeUpdateElementRank,
		Payload:        payload,
		IdempotencyKey: SerialKey(postID, elementID),
	}, nil
}

// SerialKey is both the idempotency key and the serialization key: it names the
// rows the job writes.
func SerialKey(postID, elementID int64) string {
	return fmt.Sprintf("%s:%d:%d", MessageTypeUpdateElementRank, postID, elementID)
}

func decodePayload(message queue.Message) (UpdateElementRankPayload, error) {
	var payload UpdateElementRankPayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return UpdateElementRankPayload{}, fmt.Errorf("ranking: decode payload: %w", err)
	}
	if payload.PostID <= 0 || payload.ElementID <= 0 {
		return UpdateElementRankPayload{}, fmt.Errorf(
			"ranking: payload needs a post id and element id, got post=%d element=%d",
			payload.PostID, payload.ElementID)
	}
	return payload, nil
}

// Registration describes how the worker should run element rank updates.
//
// SerialKey is a correctness requirement, not a throughput tweak. Two concurrent
// deliveries for the same element would each read the same watermark and issue
// the same aggregation over 50.7M rows; before the unique index existed they
// could also both insert, which is how the duplicate rows in production came
// about.
func (service *Service) Registration() jobs.Registration {
	return jobs.Registration{
		Type:        MessageTypeUpdateElementRank,
		Handler:     jobs.HandlerFunc(service.handle),
		Timeout:     UpdateElementRankTimeout,
		MaxAttempts: UpdateElementRankMaxAttempts,
		SerialKey: func(message queue.Message) (string, error) {
			payload, err := decodePayload(message)
			if err != nil {
				return "", err
			}
			return SerialKey(payload.PostID, payload.ElementID), nil
		},
		LaravelJob: "App\\Jobs\\UpdateElementRank",
	}
}

func (service *Service) handle(ctx context.Context, message queue.Message) error {
	payload, err := decodePayload(message)
	if err != nil {
		// A malformed payload cannot be fixed by retrying, so it is dead-lettered
		// immediately rather than consuming the attempt budget.
		return jobs.Permanent(err)
	}
	return service.UpdateElementRank(ctx, payload.PostID, payload.ElementID)
}

// Rank report contracts.
const (
	// UpdateRankReportTimeout bounds one attempt. The report reads every rank row
	// for the post and rewrites its whole report table, so it is slower than a
	// single element update while staying under the worker's 90s ceiling.
	UpdateRankReportTimeout = 75 * time.Second
	// UpdateRankReportMaxAttempts bounds redelivery. The write is a full
	// recomputation, so a retry always converges.
	UpdateRankReportMaxAttempts = 5
)

// UpdateRankReportPayload is the message body.
type UpdateRankReportPayload struct {
	PostID int64 `json:"post_id"`
}

// NewUpdateRankReportMessage builds a dispatchable message.
func NewUpdateRankReportMessage(postID int64) (queue.Message, error) {
	if postID <= 0 {
		return queue.Message{}, fmt.Errorf("ranking: post id is required, got %d", postID)
	}
	payload, err := json.Marshal(UpdateRankReportPayload{PostID: postID})
	if err != nil {
		return queue.Message{}, fmt.Errorf("ranking: encode payload: %w", err)
	}
	return queue.Message{
		Queue:          QueueDefault,
		Type:           MessageTypeUpdateRankReport,
		Payload:        payload,
		IdempotencyKey: ReportSerialKey(postID),
	}, nil
}

// ReportSerialKey names the rows the report job rewrites: every rank_report of
// one post.
func ReportSerialKey(postID int64) string {
	return fmt.Sprintf("%s:%d", MessageTypeUpdateRankReport, postID)
}

func decodeReportPayload(message queue.Message) (UpdateRankReportPayload, error) {
	var payload UpdateRankReportPayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return UpdateRankReportPayload{}, fmt.Errorf("ranking: decode payload: %w", err)
	}
	if payload.PostID <= 0 {
		return UpdateRankReportPayload{}, fmt.Errorf("ranking: payload needs a post id, got %d", payload.PostID)
	}
	return payload, nil
}

// ReportRegistration describes how the worker should run rank report updates.
//
// The serial key is the post, not the element: the job recomputes and rewrites
// every report row of one post, so two concurrent runs would interleave their
// upserts and could assign ranks from two different snapshots.
func (service *Service) ReportRegistration() jobs.Registration {
	return jobs.Registration{
		Type:        MessageTypeUpdateRankReport,
		Handler:     jobs.HandlerFunc(service.handleReport),
		Timeout:     UpdateRankReportTimeout,
		MaxAttempts: UpdateRankReportMaxAttempts,
		SerialKey: func(message queue.Message) (string, error) {
			payload, err := decodeReportPayload(message)
			if err != nil {
				return "", err
			}
			return ReportSerialKey(payload.PostID), nil
		},
		LaravelJob: "App\\Jobs\\UpdateRankReport",
	}
}

func (service *Service) handleReport(ctx context.Context, message queue.Message) error {
	payload, err := decodeReportPayload(message)
	if err != nil {
		return jobs.Permanent(err)
	}
	return service.CreateRankReports(ctx, payload.PostID)
}

// Rank history contracts.
const (
	// CreateRankHistoryTimeout bounds one attempt. A first build walks from the
	// post's creation date, which can be hundreds of days, and the thousand-votes
	// recompute scans up to 1000 rounds. Still under the worker's 90s ceiling.
	CreateRankHistoryTimeout = 80 * time.Second
	// CreateRankHistoryMaxAttempts bounds redelivery.
	CreateRankHistoryMaxAttempts = 5
)

// CreateRankHistoryPayload is the message body.
//
// StartAt is optional and only used on a first build, matching the $start
// argument threaded through CreateRankReportHistory.
type CreateRankHistoryPayload struct {
	RankReportID  int64  `json:"rank_report_id"`
	PostID        int64  `json:"post_id"`
	ElementID     int64  `json:"element_id"`
	PostCreatedAt string `json:"post_created_at"`
	StartAt       string `json:"start_at,omitempty"`
	Refresh       bool   `json:"refresh,omitempty"`
}

// NewCreateRankHistoryMessage builds a dispatchable message.
func NewCreateRankHistoryMessage(report RankReportRef, refresh bool, startAt time.Time) (queue.Message, error) {
	if report.ID <= 0 || report.PostID <= 0 || report.ElementID <= 0 {
		return queue.Message{}, fmt.Errorf("ranking: report needs id, post id and element id, got %+v", report)
	}

	body := CreateRankHistoryPayload{
		RankReportID: report.ID,
		PostID:       report.PostID,
		ElementID:    report.ElementID,
		Refresh:      refresh,
	}
	if !report.PostCreatedAt.IsZero() {
		body.PostCreatedAt = report.PostCreatedAt.Format(dateLayout)
	}
	if !startAt.IsZero() {
		body.StartAt = startAt.Format(dateLayout)
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return queue.Message{}, fmt.Errorf("ranking: encode payload: %w", err)
	}
	return queue.Message{
		Queue:          QueueRankReportHistory,
		Type:           MessageTypeCreateRankHistory,
		Payload:        payload,
		IdempotencyKey: HistorySerialKey(report.ID),
	}, nil
}

// HistorySerialKey names the rows the history builder writes: every history row of
// one report. It matches CreateRankReportHistory::uniqueId, which returns the
// rank report id, so the Go and Laravel single-flight scopes agree.
func HistorySerialKey(rankReportID int64) string {
	return fmt.Sprintf("%s:%d", MessageTypeCreateRankHistory, rankReportID)
}

func decodeHistoryPayload(message queue.Message) (CreateRankHistoryPayload, error) {
	var payload CreateRankHistoryPayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return CreateRankHistoryPayload{}, fmt.Errorf("ranking: decode payload: %w", err)
	}
	if payload.RankReportID <= 0 || payload.PostID <= 0 || payload.ElementID <= 0 {
		return CreateRankHistoryPayload{}, fmt.Errorf(
			"ranking: payload needs rank_report_id, post_id and element_id, got %+v", payload)
	}
	return payload, nil
}

func parseOptionalDate(value, field string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("ranking: %s must be %s, got %q", field, dateLayout, value)
	}
	return parsed, nil
}

// HistoryRegistration describes how the worker should build rank history.
func (service *Service) HistoryRegistration() jobs.Registration {
	return jobs.Registration{
		Type:        MessageTypeCreateRankHistory,
		Handler:     jobs.HandlerFunc(service.handleCreateHistory),
		Timeout:     CreateRankHistoryTimeout,
		MaxAttempts: CreateRankHistoryMaxAttempts,
		SerialKey: func(message queue.Message) (string, error) {
			payload, err := decodeHistoryPayload(message)
			if err != nil {
				return "", err
			}
			return HistorySerialKey(payload.RankReportID), nil
		},
		LaravelJob: "App\\Jobs\\CreateRankReportHistory",
	}
}

// handleCreateHistory builds both ranges, in the same order as
// CreateRankReportHistory::handle.
//
// A failure in the first range aborts before the second: retrying repeats both,
// and both are safe to repeat.
func (service *Service) handleCreateHistory(ctx context.Context, message queue.Message) error {
	payload, err := decodeHistoryPayload(message)
	if err != nil {
		return jobs.Permanent(err)
	}

	postCreatedAt, err := parseOptionalDate(payload.PostCreatedAt, "post_created_at")
	if err != nil {
		return jobs.Permanent(err)
	}
	startAt, err := parseOptionalDate(payload.StartAt, "start_at")
	if err != nil {
		return jobs.Permanent(err)
	}

	report := RankReportRef{
		ID:            payload.RankReportID,
		PostID:        payload.PostID,
		ElementID:     payload.ElementID,
		PostCreatedAt: postCreatedAt,
	}

	if err := service.BuildAllHistory(ctx, report, payload.Refresh, startAt); err != nil {
		return err
	}
	return service.BuildThousandVotesHistory(ctx, report, payload.Refresh)
}

// Queue message types for the rank assignment and purge passes.
const (
	// MessageTypeReorderRankHistory replaces App\Jobs\ReorderRankReportHistory:
	// it consumes the pending dates for a post and fans out one assignment per
	// (range, date).
	MessageTypeReorderRankHistory = "rank.reorder_history"
	// MessageTypeAssignRankHistory replaces App\Jobs\UpdateRankForReportHistory.
	MessageTypeAssignRankHistory = "rank.assign_history"
	// MessageTypePurgeRankHistory replaces App\Jobs\RemoveOutdateRankHistory.
	MessageTypePurgeRankHistory = "rank.purge_history"
)

// Contracts for the three passes. All run on the rank_report_history queue, as
// their Laravel counterparts do.
const (
	// ReorderRankHistoryTimeout bounds one attempt. It only reads a Redis set and
	// publishes, so it is short.
	ReorderRankHistoryTimeout = 30 * time.Second
	// AssignRankHistoryTimeout bounds one attempt. It reads and rewrites every
	// history row for one post and date, which for the largest posts is hundreds
	// of rows.
	AssignRankHistoryTimeout = 60 * time.Second
	// PurgeRankHistoryTimeout bounds one attempt. It removes at most
	// HistoryPurgeBatchSize rows.
	PurgeRankHistoryTimeout = 60 * time.Second

	rankHistoryMaxAttempts = 5
)

// ReorderRankHistoryPayload is the message body.
type ReorderRankHistoryPayload struct {
	PostID int64 `json:"post_id"`
}

// AssignRankHistoryPayload is the message body.
type AssignRankHistoryPayload struct {
	PostID    int64  `json:"post_id"`
	TimeRange string `json:"time_range"`
	StartDate string `json:"start_date"`
}

// PurgeRankHistoryPayload is the message body.
type PurgeRankHistoryPayload struct {
	PostID int64 `json:"post_id"`
}

// NewReorderRankHistoryMessage builds a dispatchable message.
func NewReorderRankHistoryMessage(postID int64) (queue.Message, error) {
	if postID <= 0 {
		return queue.Message{}, fmt.Errorf("ranking: post id is required, got %d", postID)
	}
	payload, err := json.Marshal(ReorderRankHistoryPayload{PostID: postID})
	if err != nil {
		return queue.Message{}, fmt.Errorf("ranking: encode payload: %w", err)
	}
	return queue.Message{
		Queue:          QueueRankReportHistory,
		Type:           MessageTypeReorderRankHistory,
		Payload:        payload,
		IdempotencyKey: fmt.Sprintf("%s:%d", MessageTypeReorderRankHistory, postID),
	}, nil
}

// NewAssignRankHistoryMessage builds a dispatchable message for one target.
func NewAssignRankHistoryMessage(target HistoryRankTarget) (queue.Message, error) {
	if target.PostID <= 0 {
		return queue.Message{}, fmt.Errorf("ranking: post id is required, got %d", target.PostID)
	}
	if !target.TimeRange.Valid() {
		return queue.Message{}, fmt.Errorf("ranking: unknown history time range %q", target.TimeRange)
	}

	startDate := target.StartDate.Format(dateLayout)
	payload, err := json.Marshal(AssignRankHistoryPayload{
		PostID:    target.PostID,
		TimeRange: string(target.TimeRange),
		StartDate: startDate,
	})
	if err != nil {
		return queue.Message{}, fmt.Errorf("ranking: encode payload: %w", err)
	}
	return queue.Message{
		Queue:   QueueRankReportHistory,
		Type:    MessageTypeAssignRankHistory,
		Payload: payload,
		// The natural key of the work: one post, range and date.
		IdempotencyKey: AssignSerialKey(target.PostID, target.TimeRange, startDate),
	}, nil
}

// NewPurgeRankHistoryMessage builds a dispatchable message.
func NewPurgeRankHistoryMessage(postID int64) (queue.Message, error) {
	if postID <= 0 {
		return queue.Message{}, fmt.Errorf("ranking: post id is required, got %d", postID)
	}
	payload, err := json.Marshal(PurgeRankHistoryPayload{PostID: postID})
	if err != nil {
		return queue.Message{}, fmt.Errorf("ranking: encode payload: %w", err)
	}
	return queue.Message{
		Queue:          QueueRankReportHistory,
		Type:           MessageTypePurgeRankHistory,
		Payload:        payload,
		IdempotencyKey: fmt.Sprintf("%s:%d", MessageTypePurgeRankHistory, postID),
	}, nil
}

// AssignSerialKey names the rows one assignment rewrites.
func AssignSerialKey(postID int64, timeRange HistoryTimeRange, startDate string) string {
	return fmt.Sprintf("%s:%d:%s:%s", MessageTypeAssignRankHistory, postID, timeRange, startDate)
}

// ReorderRegistration describes the pending-dates fan-out.
//
// Serialised per post so two reorders cannot both pull the set and each fan out
// half the dates.
func (service *Service) ReorderRegistration(dispatcher Dispatcher) jobs.Registration {
	return jobs.Registration{
		Type:        MessageTypeReorderRankHistory,
		Timeout:     ReorderRankHistoryTimeout,
		MaxAttempts: rankHistoryMaxAttempts,
		Handler: jobs.HandlerFunc(func(ctx context.Context, message queue.Message) error {
			var payload ReorderRankHistoryPayload
			if err := json.Unmarshal(message.Payload, &payload); err != nil {
				return jobs.Permanent(fmt.Errorf("ranking: decode payload: %w", err))
			}
			if payload.PostID <= 0 {
				return jobs.Permanent(fmt.Errorf("ranking: payload needs a post id, got %d", payload.PostID))
			}
			return service.reorderAndDispatch(ctx, payload.PostID, dispatcher)
		}),
		SerialKey: func(message queue.Message) (string, error) {
			var payload ReorderRankHistoryPayload
			if err := json.Unmarshal(message.Payload, &payload); err != nil {
				return "", fmt.Errorf("ranking: decode payload: %w", err)
			}
			if payload.PostID <= 0 {
				return "", fmt.Errorf("ranking: payload needs a post id, got %d", payload.PostID)
			}
			return fmt.Sprintf("%s:%d", MessageTypeReorderRankHistory, payload.PostID), nil
		},
		LaravelJob: "App\\Jobs\\ReorderRankReportHistory",
	}
}

// reorderAndDispatch pulls the pending dates and publishes one assignment each.
//
// The dates are already out of the Redis set by the time the publish happens, so a
// failure here loses them. They are put back before returning the error, which is
// why the publish is not fire-and-forget.
func (service *Service) reorderAndDispatch(ctx context.Context, postID int64, dispatcher Dispatcher) error {
	targets, err := service.ReorderHistoryRanks(ctx, postID)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return nil
	}

	messages := make([]queue.Message, 0, len(targets))
	for _, target := range targets {
		message, err := NewAssignRankHistoryMessage(target)
		if err != nil {
			return jobs.Permanent(err)
		}
		messages = append(messages, message)
	}

	if err := dispatcher.Publish(ctx, messages...); err != nil {
		// Pull already emptied the set, so the dates would be lost. Put them back
		// so a retry, or the next reorder, still covers them.
		service.restorePendingDates(ctx, postID, targets)
		return fmt.Errorf("ranking: dispatch %d rank assignments for post %d: %w", len(messages), postID, err)
	}
	return nil
}

func (service *Service) restorePendingDates(ctx context.Context, postID int64, targets []HistoryRankTarget) {
	byRange := make(map[HistoryTimeRange][]string, 2)
	for _, target := range targets {
		byRange[target.TimeRange] = append(byRange[target.TimeRange], target.StartDate.Format(dateLayout))
	}
	for timeRange, dates := range byRange {
		if err := service.pending.Add(ctx, postID, timeRange, dates); err != nil {
			service.logger.Error("rank_history_pending_dates_lost",
				"post_id", postID, "time_range", string(timeRange), "dates", len(dates), "error", err)
		}
	}
}

// AssignRegistration describes one rank assignment.
func (service *Service) AssignRegistration() jobs.Registration {
	decode := func(message queue.Message) (AssignRankHistoryPayload, error) {
		var payload AssignRankHistoryPayload
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return payload, fmt.Errorf("ranking: decode payload: %w", err)
		}
		if payload.PostID <= 0 {
			return payload, fmt.Errorf("ranking: payload needs a post id, got %d", payload.PostID)
		}
		if !HistoryTimeRange(payload.TimeRange).Valid() {
			return payload, fmt.Errorf("ranking: unknown history time range %q", payload.TimeRange)
		}
		if _, err := time.Parse(dateLayout, payload.StartDate); err != nil {
			return payload, fmt.Errorf("ranking: start_date must be %s, got %q", dateLayout, payload.StartDate)
		}
		return payload, nil
	}

	return jobs.Registration{
		Type:        MessageTypeAssignRankHistory,
		Timeout:     AssignRankHistoryTimeout,
		MaxAttempts: rankHistoryMaxAttempts,
		Handler: jobs.HandlerFunc(func(ctx context.Context, message queue.Message) error {
			payload, err := decode(message)
			if err != nil {
				return jobs.Permanent(err)
			}
			startDate, _ := time.Parse(dateLayout, payload.StartDate)
			return service.AssignHistoryRanks(ctx, payload.PostID, HistoryTimeRange(payload.TimeRange), startDate)
		}),
		SerialKey: func(message queue.Message) (string, error) {
			payload, err := decode(message)
			if err != nil {
				return "", err
			}
			return AssignSerialKey(payload.PostID, HistoryTimeRange(payload.TimeRange), payload.StartDate), nil
		},
		LaravelJob: "App\\Jobs\\UpdateRankForReportHistory",
	}
}

// PurgeRegistration describes the retention pass.
func (service *Service) PurgeRegistration() jobs.Registration {
	decode := func(message queue.Message) (PurgeRankHistoryPayload, error) {
		var payload PurgeRankHistoryPayload
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return payload, fmt.Errorf("ranking: decode payload: %w", err)
		}
		if payload.PostID <= 0 {
			return payload, fmt.Errorf("ranking: payload needs a post id, got %d", payload.PostID)
		}
		return payload, nil
	}

	return jobs.Registration{
		Type:        MessageTypePurgeRankHistory,
		Timeout:     PurgeRankHistoryTimeout,
		MaxAttempts: rankHistoryMaxAttempts,
		Handler: jobs.HandlerFunc(func(ctx context.Context, message queue.Message) error {
			payload, err := decode(message)
			if err != nil {
				return jobs.Permanent(err)
			}
			_, err = service.RemoveOutdatedHistory(ctx, payload.PostID)
			return err
		}),
		SerialKey: func(message queue.Message) (string, error) {
			payload, err := decode(message)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s:%d", MessageTypePurgeRankHistory, payload.PostID), nil
		},
		LaravelJob: "App\\Jobs\\RemoveOutdateRankHistory",
	}
}

// Dispatcher publishes follow-up messages. Kept local to this package so the
// reorder handler can fan out without importing the worker.
type Dispatcher interface {
	Publish(ctx context.Context, messages ...queue.Message) error
}

// Queue message types for the daily sweeps.
const (
	// MessageTypeSweepPostHistory replaces App\Console\Commands\
	// MakeRankReportHistoryCommand.
	MessageTypeSweepPostHistory = "rank.sweep_post_history"
	// MessageTypeBuildPostHistory replaces App\Jobs\CreateAndUpdateRankHistory.
	MessageTypeBuildPostHistory = "rank.build_post_history"
	// MessageTypeSweepPurgeHistory replaces
	// RankReportScheduleExecutor::removeOutdateRankReportHistory.
	MessageTypeSweepPurgeHistory = "rank.sweep_purge_history"
)

// Contracts for the sweeps. A sweep walks every post, so it is the longest-running
// of these jobs while still doing nothing but reads and publishes.
const (
	SweepPostHistoryTimeout  = 85 * time.Second
	BuildPostHistoryTimeout  = 60 * time.Second
	SweepPurgeHistoryTimeout = 85 * time.Second
)

// SweepPostHistoryPayload is the message body.
type SweepPostHistoryPayload struct {
	// Refresh maps to the command's --refresh flag: it rebuilds history from
	// scratch instead of resuming.
	Refresh bool `json:"refresh,omitempty"`
}

// BuildPostHistoryPayload is the message body.
type BuildPostHistoryPayload struct {
	PostID        int64  `json:"post_id"`
	PostCreatedAt string `json:"post_created_at,omitempty"`
	Refresh       bool   `json:"refresh,omitempty"`
}

// NewSweepPostHistoryMessage builds a dispatchable message.
func NewSweepPostHistoryMessage(refresh bool) (queue.Message, error) {
	payload, err := json.Marshal(SweepPostHistoryPayload{Refresh: refresh})
	if err != nil {
		return queue.Message{}, fmt.Errorf("ranking: encode payload: %w", err)
	}
	return queue.Message{
		Queue:          QueueRankReportHistory,
		Type:           MessageTypeSweepPostHistory,
		Payload:        payload,
		IdempotencyKey: MessageTypeSweepPostHistory,
	}, nil
}

// NewBuildPostHistoryMessage builds a dispatchable message for one post.
func NewBuildPostHistoryMessage(post PostRef, refresh bool) (queue.Message, error) {
	if post.ID <= 0 {
		return queue.Message{}, fmt.Errorf("ranking: post id is required, got %d", post.ID)
	}
	body := BuildPostHistoryPayload{PostID: post.ID, Refresh: refresh}
	if !post.CreatedAt.IsZero() {
		body.PostCreatedAt = post.CreatedAt.Format(dateLayout)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return queue.Message{}, fmt.Errorf("ranking: encode payload: %w", err)
	}
	return queue.Message{
		Queue:          QueueRankReportHistory,
		Type:           MessageTypeBuildPostHistory,
		Payload:        payload,
		IdempotencyKey: fmt.Sprintf("%s:%d", MessageTypeBuildPostHistory, post.ID),
	}, nil
}

// NewSweepPurgeHistoryMessage builds a dispatchable message.
func NewSweepPurgeHistoryMessage() (queue.Message, error) {
	payload, err := json.Marshal(struct{}{})
	if err != nil {
		return queue.Message{}, fmt.Errorf("ranking: encode payload: %w", err)
	}
	return queue.Message{
		Queue:          QueueRankReportHistory,
		Type:           MessageTypeSweepPurgeHistory,
		Payload:        payload,
		IdempotencyKey: MessageTypeSweepPurgeHistory,
	}, nil
}

// SweepPostHistoryRegistration describes the daily history sweep.
//
// Serialised globally: two concurrent sweeps would walk the same posts and publish
// the same builds twice.
func (service *Service) SweepPostHistoryRegistration(dispatcher Dispatcher) jobs.Registration {
	return jobs.Registration{
		Type:        MessageTypeSweepPostHistory,
		Timeout:     SweepPostHistoryTimeout,
		MaxAttempts: rankHistoryMaxAttempts,
		Handler: jobs.HandlerFunc(func(ctx context.Context, message queue.Message) error {
			var payload SweepPostHistoryPayload
			if len(message.Payload) > 0 {
				if err := json.Unmarshal(message.Payload, &payload); err != nil {
					return jobs.Permanent(fmt.Errorf("ranking: decode payload: %w", err))
				}
			}
			_, err := service.SweepPostHistory(ctx, payload.Refresh, dispatcher)
			return err
		}),
		SerialKey:  func(queue.Message) (string, error) { return MessageTypeSweepPostHistory, nil },
		LaravelJob: "App\\Console\\Commands\\MakeRankReportHistoryCommand",
	}
}

// BuildPostHistoryRegistration describes one post's history fan-out.
func (service *Service) BuildPostHistoryRegistration(dispatcher Dispatcher) jobs.Registration {
	decode := func(message queue.Message) (BuildPostHistoryPayload, error) {
		var payload BuildPostHistoryPayload
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return payload, fmt.Errorf("ranking: decode payload: %w", err)
		}
		if payload.PostID <= 0 {
			return payload, fmt.Errorf("ranking: payload needs a post id, got %d", payload.PostID)
		}
		return payload, nil
	}

	return jobs.Registration{
		Type:        MessageTypeBuildPostHistory,
		Timeout:     BuildPostHistoryTimeout,
		MaxAttempts: rankHistoryMaxAttempts,
		Handler: jobs.HandlerFunc(func(ctx context.Context, message queue.Message) error {
			payload, err := decode(message)
			if err != nil {
				return jobs.Permanent(err)
			}
			createdAt, err := parseOptionalDate(payload.PostCreatedAt, "post_created_at")
			if err != nil {
				return jobs.Permanent(err)
			}
			return service.BuildPostHistory(ctx,
				PostRef{ID: payload.PostID, CreatedAt: createdAt}, payload.Refresh, dispatcher)
		}),
		SerialKey: func(message queue.Message) (string, error) {
			payload, err := decode(message)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s:%d", MessageTypeBuildPostHistory, payload.PostID), nil
		},
		LaravelJob: "App\\Jobs\\CreateAndUpdateRankHistory",
	}
}

// SweepPurgeHistoryRegistration describes the daily retention sweep.
func (service *Service) SweepPurgeHistoryRegistration(dispatcher Dispatcher) jobs.Registration {
	return jobs.Registration{
		Type:        MessageTypeSweepPurgeHistory,
		Timeout:     SweepPurgeHistoryTimeout,
		MaxAttempts: rankHistoryMaxAttempts,
		Handler: jobs.HandlerFunc(func(ctx context.Context, _ queue.Message) error {
			_, err := service.SweepPurgeHistory(ctx, dispatcher)
			return err
		}),
		SerialKey:  func(queue.Message) (string, error) { return MessageTypeSweepPurgeHistory, nil },
		LaravelJob: "App\\ScheduleExecutor\\RankReportScheduleExecutor",
	}
}
