package ingest

import (
	"context"
	"fmt"

	"2pick.app/backend/internal/media"
	"2pick.app/backend/internal/queue"
)

// QueueThumbnailer queues the video thumbnail a new element needs, which is what
// App\Listeners\MakeVideoThumbnail did on VideoElementCreated.
//
// Images are deliberately not queued here: ImageElementCreated had no listeners, and the
// make-thumbnails schedule — already ported and already running — sweeps up images that
// have no thumbnail yet.
type QueueThumbnailer struct {
	publisher *queue.Publisher
}

func NewQueueThumbnailer(publisher *queue.Publisher) (*QueueThumbnailer, error) {
	if publisher == nil {
		return nil, fmt.Errorf("ingest: publisher is required")
	}
	return &QueueThumbnailer{publisher: publisher}, nil
}

func (thumbnailer *QueueThumbnailer) VideoThumbnail(ctx context.Context, elementID int64) error {
	message, err := media.NewVideoThumbnailMessage(elementID)
	if err != nil {
		return fmt.Errorf("ingest: build video thumbnail for %d: %w", elementID, err)
	}
	return thumbnailer.publisher.Publish(ctx, message)
}
