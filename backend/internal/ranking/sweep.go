package ranking

import (
	"context"
	"fmt"
	"time"

	"2pick.app/backend/internal/queue"
)

// PostChunkSize matches the chunkById(300) both schedules use.
const PostChunkSize = 300

// PostRef is the minimum needed to fan work out over a post.
type PostRef struct {
	ID        int64
	CreatedAt time.Time
}

// ReportRef names one rank_report of a post.
type ReportRef struct {
	ID        int64
	ElementID int64
}

// PostRepository enumerates posts and their reports for the daily sweeps.
type PostRepository interface {
	// Posts returns posts with id greater than afterID, ordered by id, capped at
	// limit. Cursor paging by id is what chunkById does, and it stays correct while
	// rows are being written.
	Posts(ctx context.Context, afterID int64, limit int) ([]PostRef, error)
	// PostsIncludingDeleted is the same but keeps soft-deleted posts, which the
	// purge sweep needs: their history still has to age out.
	PostsIncludingDeleted(ctx context.Context, afterID int64, limit int) ([]PostRef, error)
	// ReportsForPost returns the post's rank_reports.
	ReportsForPost(ctx context.Context, postID int64) ([]ReportRef, error)
}

// BuildPostHistory fans one post's rank reports out into history builds.
//
// Port of App\Jobs\CreateAndUpdateRankHistory. It dispatches one history build per
// rank_report, then a reorder for the post, then clears the freshness flag.
//
// The start date is yesterday, matching today()->subDays(1) in the original: the
// walk resumes from there rather than from the post's creation.
//
// The job counter the original maintains in the cache is not ported. Its only
// consumer was ReorderRankReportHistory's self-rescheduling loop, which this port
// also drops; see the note on ReorderHistoryRanks.
func (service *Service) BuildPostHistory(
	ctx context.Context,
	post PostRef,
	refresh bool,
	dispatcher Dispatcher,
) error {
	if service.posts == nil {
		return fmt.Errorf("ranking: post repository is not configured")
	}
	if post.ID <= 0 {
		return fmt.Errorf("ranking: post id is required, got %d", post.ID)
	}

	reports, err := service.posts.ReportsForPost(ctx, post.ID)
	if err != nil {
		return fmt.Errorf("ranking: reports for post %d: %w", post.ID, err)
	}

	startAt := service.now().AddDate(0, 0, -1)
	messages := make([]queue.Message, 0, len(reports)+1)
	for _, report := range reports {
		message, err := NewCreateRankHistoryMessage(RankReportRef{
			ID:            report.ID,
			PostID:        post.ID,
			ElementID:     report.ElementID,
			PostCreatedAt: post.CreatedAt,
		}, refresh, startAt)
		if err != nil {
			// One unusable report must not stop the rest of the post.
			service.logger.Warn("rank_history_build_skipped_report",
				"post_id", post.ID, "rank_report_id", report.ID, "error", err)
			continue
		}
		messages = append(messages, message)
	}

	// The reorder is queued last so it is behind the builds it depends on. FIFO
	// ordering per queue makes that hold for a single consumer; with several, a
	// reorder that runs early simply finds fewer pending dates and the next one
	// picks up the rest.
	reorder, err := NewReorderRankHistoryMessage(post.ID)
	if err != nil {
		return err
	}
	messages = append(messages, reorder)

	if err := dispatcher.Publish(ctx, messages...); err != nil {
		return fmt.Errorf("ranking: dispatch history builds for post %d: %w", post.ID, err)
	}

	// Cleared only after the work is queued. Clearing first would drop the flag on
	// a post whose builds were never dispatched.
	if service.freshness != nil {
		if err := service.freshness.Clear(ctx, post.ID); err != nil {
			// The work is already queued, so this is not worth failing the job for:
			// the next sweep would simply rebuild the same post.
			service.logger.Warn("rank_history_freshness_not_cleared",
				"post_id", post.ID, "error", err)
		}
	}

	service.logger.Info("rank_history_post_dispatched",
		"post_id", post.ID,
		"reports", len(reports),
		"messages", len(messages),
		"start_at", startAt.Format(dateLayout),
		"refresh", refresh,
	)
	return nil
}

// SweepPostHistory dispatches a history build for every post whose ranks changed.
//
// Port of MakeRankReportHistoryCommand. Only flagged posts are processed, which is
// what keeps a daily run from rebuilding all 6,201 posts.
func (service *Service) SweepPostHistory(ctx context.Context, refresh bool, dispatcher Dispatcher) (int, error) {
	if service.posts == nil {
		return 0, fmt.Errorf("ranking: post repository is not configured")
	}
	if service.freshness == nil {
		return 0, fmt.Errorf("ranking: freshness store is not configured")
	}

	dispatched := 0
	scanned := 0
	var afterID int64

	for {
		posts, err := service.posts.Posts(ctx, afterID, PostChunkSize)
		if err != nil {
			return dispatched, fmt.Errorf("ranking: list posts after %d: %w", afterID, err)
		}
		if len(posts) == 0 {
			break
		}

		for _, post := range posts {
			scanned++
			afterID = post.ID

			flagged, err := service.freshness.NeedsRebuild(ctx, post.ID)
			if err != nil {
				return dispatched, err
			}
			if !flagged {
				continue
			}

			message, err := NewBuildPostHistoryMessage(post, refresh)
			if err != nil {
				service.logger.Warn("rank_history_sweep_skipped_post", "post_id", post.ID, "error", err)
				continue
			}
			if err := dispatcher.Publish(ctx, message); err != nil {
				return dispatched, fmt.Errorf("ranking: dispatch history build for post %d: %w", post.ID, err)
			}
			dispatched++
		}
	}

	service.logger.Info("rank_history_sweep_completed",
		"posts_scanned", scanned, "posts_dispatched", dispatched, "refresh", refresh)
	return dispatched, nil
}

// SweepPurgeHistory dispatches a retention purge for every post.
//
// Port of RankReportScheduleExecutor::removeOutdateRankReportHistory. Soft-deleted
// posts are included: their history still has to age out.
func (service *Service) SweepPurgeHistory(ctx context.Context, dispatcher Dispatcher) (int, error) {
	if service.posts == nil {
		return 0, fmt.Errorf("ranking: post repository is not configured")
	}

	dispatched := 0
	var afterID int64

	for {
		posts, err := service.posts.PostsIncludingDeleted(ctx, afterID, PostChunkSize)
		if err != nil {
			return dispatched, fmt.Errorf("ranking: list posts after %d: %w", afterID, err)
		}
		if len(posts) == 0 {
			break
		}

		messages := make([]queue.Message, 0, len(posts))
		for _, post := range posts {
			afterID = post.ID
			message, err := NewPurgeRankHistoryMessage(post.ID)
			if err != nil {
				service.logger.Warn("rank_history_purge_skipped_post", "post_id", post.ID, "error", err)
				continue
			}
			messages = append(messages, message)
		}
		if len(messages) == 0 {
			continue
		}
		if err := dispatcher.Publish(ctx, messages...); err != nil {
			return dispatched, fmt.Errorf("ranking: dispatch purges: %w", err)
		}
		dispatched += len(messages)
	}

	service.logger.Info("rank_history_purge_sweep_completed", "posts_dispatched", dispatched)
	return dispatched, nil
}
