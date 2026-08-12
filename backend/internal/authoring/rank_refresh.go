package authoring

import (
	"context"
	"fmt"

	"2pick.app/backend/internal/queue"
	"2pick.app/backend/internal/ranking"
)

// QueueRankRefresher queues the rank-report rebuild that a deletion invalidates.
//
// The original dispatched UpdateRankReport with ->delay(10), a pause it never explained.
// The plausible reason is that Laravel dispatched inside the request and the listener
// could otherwise run before the transaction that deleted the rows had committed — the
// same after-commit defect this port already has a regression test for. Here the message
// is published after the transaction returns, so the delay has nothing left to protect
// against and is not reproduced.
type QueueRankRefresher struct {
	publisher *queue.Publisher
}

func NewQueueRankRefresher(publisher *queue.Publisher) (*QueueRankRefresher, error) {
	if publisher == nil {
		return nil, fmt.Errorf("authoring: publisher is required")
	}
	return &QueueRankRefresher{publisher: publisher}, nil
}

func (refresher *QueueRankRefresher) RefreshPostRank(ctx context.Context, postID int64) error {
	message, err := ranking.NewUpdateRankReportMessage(postID)
	if err != nil {
		return fmt.Errorf("authoring: build rank refresh for post %d: %w", postID, err)
	}
	return refresher.publisher.Publish(ctx, message)
}
