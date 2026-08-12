package media

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"2pick.app/backend/internal/jobs"
	"2pick.app/backend/internal/queue"
)

// Queue message types.
const (
	// MessageTypeMakeImageThumbnail replaces App\Jobs\ResizeElementImage together
	// with the service that dispatched it.
	MessageTypeMakeImageThumbnail = "media.make_image_thumbnail"
	// MessageTypeMakeVideoThumbnail replaces App\Jobs\MakeVideoThumbnail, which is
	// synchronous in Laravel.
	MessageTypeMakeVideoThumbnail = "media.make_video_thumbnail"
	// MessageTypeSweepThumbnails replaces the "Make Thumbnails" schedule: it finds
	// elements missing a derivative and fans out one job each.
	MessageTypeSweepThumbnails = "media.sweep_thumbnails"
	// MessageTypeRemoveDeletedFiles replaces the "Remove Unused Images" schedule.
	MessageTypeRemoveDeletedFiles = "media.remove_deleted_files"
)

// QueueMedia carries the media work.
//
// Laravel puts ResizeElementImage on the default queue, but these jobs run ffmpeg
// and upload to S3, so a backlog of them would sit in front of the ranking jobs.
// A dedicated queue keeps the two from starving each other.
const QueueMedia = "media"

// Contracts. None of the Laravel classes declares any of these.
const (
	// ImageThumbnailTimeout covers a fetch, a probe, an encode and an upload. The
	// fetch alone is bounded at FetchTimeout and the encode at TranscodeTimeout.
	ImageThumbnailTimeout = 85 * time.Second
	// VideoThumbnailTimeout covers ffmpeg reading a remote video and an upload.
	VideoThumbnailTimeout = 85 * time.Second
	// SweepTimeout covers one query and the fan-out publish.
	SweepTimeout = 60 * time.Second
	// RemoveDeletedFilesTimeout covers up to RemoveDeletedFilesLimit elements, each
	// with a handful of head and delete calls.
	RemoveDeletedFilesTimeout = 85 * time.Second

	// mediaMaxAttempts bounds redelivery. A transient fetch or upload failure is
	// worth retrying; a permanently broken source ends up dead-lettered.
	mediaMaxAttempts = 3
)

// Sweep limits, matching the schedule arguments in Console\Kernel.
const (
	// SweepThumbnailLimit matches ThumbnailExecutor::makeElementThumbnails(300),
	// the value the hourly schedule passes.
	SweepThumbnailLimit = 300
	// RemoveDeletedFilesLimit matches removeDeletedFiles(1000).
	RemoveDeletedFilesLimit = 1000
)

// Dispatcher publishes the fan-out messages.
type Dispatcher interface {
	Publish(ctx context.Context, messages ...queue.Message) error
}

// ImageThumbnailPayload is the message body.
type ImageThumbnailPayload struct {
	ElementID int64  `json:"element_id"`
	Column    string `json:"column"`
}

// VideoThumbnailPayload is the message body.
type VideoThumbnailPayload struct {
	ElementID int64 `json:"element_id"`
}

// SweepPayload is the message body. An empty Column means both derivatives.
type SweepPayload struct {
	Column string `json:"column,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// RemoveDeletedFilesPayload is the message body.
type RemoveDeletedFilesPayload struct {
	Limit int `json:"limit,omitempty"`
}

// NewImageThumbnailMessage builds a dispatchable message.
func NewImageThumbnailMessage(elementID int64, column string) (queue.Message, error) {
	if elementID <= 0 {
		return queue.Message{}, fmt.Errorf("media: element id is required, got %d", elementID)
	}
	if _, err := SpecForColumn(column); err != nil {
		return queue.Message{}, err
	}
	payload, err := json.Marshal(ImageThumbnailPayload{ElementID: elementID, Column: column})
	if err != nil {
		return queue.Message{}, fmt.Errorf("media: encode payload: %w", err)
	}
	return queue.Message{
		Queue:   QueueMedia,
		Type:    MessageTypeMakeImageThumbnail,
		Payload: payload,
		// One derivative of one element is the unit of work, so that is the key.
		IdempotencyKey: imageSerialKey(elementID, column),
	}, nil
}

// NewVideoThumbnailMessage builds a dispatchable message.
func NewVideoThumbnailMessage(elementID int64) (queue.Message, error) {
	if elementID <= 0 {
		return queue.Message{}, fmt.Errorf("media: element id is required, got %d", elementID)
	}
	payload, err := json.Marshal(VideoThumbnailPayload{ElementID: elementID})
	if err != nil {
		return queue.Message{}, fmt.Errorf("media: encode payload: %w", err)
	}
	return queue.Message{
		Queue:          QueueMedia,
		Type:           MessageTypeMakeVideoThumbnail,
		Payload:        payload,
		IdempotencyKey: fmt.Sprintf("%s:%d", MessageTypeMakeVideoThumbnail, elementID),
	}, nil
}

// NewSweepMessage builds a dispatchable message for the hourly sweep.
func NewSweepMessage(column string, limit int) (queue.Message, error) {
	if column != "" {
		if _, err := SpecForColumn(column); err != nil {
			return queue.Message{}, err
		}
	}
	if limit <= 0 {
		limit = SweepThumbnailLimit
	}
	payload, err := json.Marshal(SweepPayload{Column: column, Limit: limit})
	if err != nil {
		return queue.Message{}, fmt.Errorf("media: encode payload: %w", err)
	}
	key := MessageTypeSweepThumbnails
	if column != "" {
		key += ":" + column
	}
	return queue.Message{
		Queue:          QueueMedia,
		Type:           MessageTypeSweepThumbnails,
		Payload:        payload,
		IdempotencyKey: key,
	}, nil
}

// NewRemoveDeletedFilesMessage builds a dispatchable message.
func NewRemoveDeletedFilesMessage(limit int) (queue.Message, error) {
	if limit <= 0 {
		limit = RemoveDeletedFilesLimit
	}
	payload, err := json.Marshal(RemoveDeletedFilesPayload{Limit: limit})
	if err != nil {
		return queue.Message{}, fmt.Errorf("media: encode payload: %w", err)
	}
	return queue.Message{
		Queue:          QueueMedia,
		Type:           MessageTypeRemoveDeletedFiles,
		Payload:        payload,
		IdempotencyKey: MessageTypeRemoveDeletedFiles,
	}, nil
}

func imageSerialKey(elementID int64, column string) string {
	return fmt.Sprintf("%s:%d:%s", MessageTypeMakeImageThumbnail, elementID, column)
}

// ImageRegistration describes one image derivative job.
//
// Serialised per element and column: two concurrent runs would each fetch, encode
// and upload, then race to write the column, leaving one uploaded object orphaned.
func (service *ThumbnailService) ImageRegistration() jobs.Registration {
	decode := func(message queue.Message) (ImageThumbnailPayload, ThumbnailSpec, error) {
		var payload ImageThumbnailPayload
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return payload, ThumbnailSpec{}, fmt.Errorf("media: decode payload: %w", err)
		}
		if payload.ElementID <= 0 {
			return payload, ThumbnailSpec{}, fmt.Errorf("media: payload needs an element id, got %d", payload.ElementID)
		}
		spec, err := SpecForColumn(payload.Column)
		if err != nil {
			return payload, ThumbnailSpec{}, err
		}
		return payload, spec, nil
	}

	return jobs.Registration{
		Type:        MessageTypeMakeImageThumbnail,
		Timeout:     ImageThumbnailTimeout,
		MaxAttempts: mediaMaxAttempts,
		Handler: jobs.HandlerFunc(func(ctx context.Context, message queue.Message) error {
			payload, spec, err := decode(message)
			if err != nil {
				return jobs.Permanent(err)
			}
			return service.MakeImageThumbnail(ctx, payload.ElementID, spec)
		}),
		SerialKey: func(message queue.Message) (string, error) {
			payload, _, err := decode(message)
			if err != nil {
				return "", err
			}
			return imageSerialKey(payload.ElementID, payload.Column), nil
		},
		LaravelJob: "App\\Jobs\\ResizeElementImage",
	}
}

// VideoRegistration describes the video frame job.
func (service *ThumbnailService) VideoRegistration() jobs.Registration {
	decode := func(message queue.Message) (VideoThumbnailPayload, error) {
		var payload VideoThumbnailPayload
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return payload, fmt.Errorf("media: decode payload: %w", err)
		}
		if payload.ElementID <= 0 {
			return payload, fmt.Errorf("media: payload needs an element id, got %d", payload.ElementID)
		}
		return payload, nil
	}

	return jobs.Registration{
		Type:        MessageTypeMakeVideoThumbnail,
		Timeout:     VideoThumbnailTimeout,
		MaxAttempts: mediaMaxAttempts,
		Handler: jobs.HandlerFunc(func(ctx context.Context, message queue.Message) error {
			payload, err := decode(message)
			if err != nil {
				return jobs.Permanent(err)
			}
			return service.MakeVideoThumbnail(ctx, payload.ElementID)
		}),
		SerialKey: func(message queue.Message) (string, error) {
			payload, err := decode(message)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s:%d", MessageTypeMakeVideoThumbnail, payload.ElementID), nil
		},
		LaravelJob: "App\\Jobs\\MakeVideoThumbnail",
	}
}

// SweepRegistration describes the hourly thumbnail sweep.
//
// It only finds work and fans out; the derivative jobs do the fetching and
// encoding. That keeps one long-running sweep from holding a worker slot for the
// whole batch, which is what ThumbnailExecutor does by looping inline.
func (service *ThumbnailService) SweepRegistration(dispatcher Dispatcher) jobs.Registration {
	return jobs.Registration{
		Type:        MessageTypeSweepThumbnails,
		Timeout:     SweepTimeout,
		MaxAttempts: mediaMaxAttempts,
		Handler: jobs.HandlerFunc(func(ctx context.Context, message queue.Message) error {
			var payload SweepPayload
			if len(message.Payload) > 0 {
				if err := json.Unmarshal(message.Payload, &payload); err != nil {
					return jobs.Permanent(fmt.Errorf("media: decode payload: %w", err))
				}
			}
			limit := payload.Limit
			if limit <= 0 {
				limit = SweepThumbnailLimit
			}

			specs := []ThumbnailSpec{LowThumbnailSpec(), MediumThumbnailSpec()}
			if payload.Column != "" {
				spec, err := SpecForColumn(payload.Column)
				if err != nil {
					return jobs.Permanent(err)
				}
				specs = []ThumbnailSpec{spec}
			}
			return service.sweepAndDispatch(ctx, specs, limit, dispatcher)
		}),
		SerialKey: func(queue.Message) (string, error) {
			// One sweep at a time regardless of column: two concurrent sweeps would
			// find the same elements and publish duplicate work.
			return MessageTypeSweepThumbnails, nil
		},
		LaravelJob: "App\\ScheduleExecutor\\ThumbnailExecutor",
	}
}

func (service *ThumbnailService) sweepAndDispatch(
	ctx context.Context, specs []ThumbnailSpec, limit int, dispatcher Dispatcher,
) error {
	published := 0
	for _, spec := range specs {
		elements, err := service.PendingThumbnails(ctx, spec, limit)
		if err != nil {
			return err
		}
		if len(elements) == 0 {
			continue
		}

		messages := make([]queue.Message, 0, len(elements))
		for _, element := range elements {
			message, err := NewImageThumbnailMessage(element.ID, spec.Column)
			if err != nil {
				// A row that cannot be turned into a message is skipped rather than
				// failing the sweep and stalling every other element.
				service.logger.Warn("media_sweep_skipped_element",
					"element_id", element.ID, "column", spec.Column, "error", err)
				continue
			}
			messages = append(messages, message)
		}
		if len(messages) == 0 {
			continue
		}
		if err := dispatcher.Publish(ctx, messages...); err != nil {
			return fmt.Errorf("media: dispatch %d %s jobs: %w", len(messages), spec.Column, err)
		}
		published += len(messages)

		service.logger.Info("media_sweep_dispatched",
			"column", spec.Column, "candidates", len(elements), "dispatched", len(messages), "limit", limit)
	}

	if published == 0 {
		service.logger.Info("media_sweep_found_nothing", "limit", limit)
	}
	return nil
}

// RemoveDeletedFilesRegistration describes the storage cleanup.
func (service *ThumbnailService) RemoveDeletedFilesRegistration() jobs.Registration {
	return jobs.Registration{
		Type:        MessageTypeRemoveDeletedFiles,
		Timeout:     RemoveDeletedFilesTimeout,
		MaxAttempts: mediaMaxAttempts,
		Handler: jobs.HandlerFunc(func(ctx context.Context, message queue.Message) error {
			var payload RemoveDeletedFilesPayload
			if len(message.Payload) > 0 {
				if err := json.Unmarshal(message.Payload, &payload); err != nil {
					return jobs.Permanent(fmt.Errorf("media: decode payload: %w", err))
				}
			}
			limit := payload.Limit
			if limit <= 0 {
				limit = RemoveDeletedFilesLimit
			}
			cleaned, err := service.RemoveDeletedElementFiles(ctx, limit)
			if err != nil {
				return err
			}
			service.logger.Info("media_remove_deleted_files_completed",
				"elements_cleaned", cleaned, "limit", limit,
				// A full batch means more remain and one run per hour will not drain
				// them.
				"batch_full", cleaned == limit)
			return nil
		}),
		SerialKey: func(queue.Message) (string, error) {
			// One cleanup at a time: two runs would both list the same elements and
			// race on the same object deletions.
			return MessageTypeRemoveDeletedFiles, nil
		},
		LaravelJob: "App\\ScheduleExecutor\\ElementScheduleExecutor",
	}
}
