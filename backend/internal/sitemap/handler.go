package sitemap

import (
	"context"
	"time"

	"2pick.app/backend/internal/jobs"
	"2pick.app/backend/internal/queue"
)

// MessageTypeGenerate replaces App\Console\Commands\GenerateSitemap.
const MessageTypeGenerate = "sitemap.generate"

// QueueSitemap carries the generation. It sits with the other low-urgency work:
// nothing waits on a sitemap.
const QueueSitemap = "low"

const (
	// GenerateTimeout bounds one attempt. The walk is a few hundred rows plus one
	// upload.
	GenerateTimeout = 85 * time.Second
	// GenerateMaxAttempts bounds redelivery. Regenerating is a full rebuild, so a
	// retry always converges.
	GenerateMaxAttempts = 3
)

// NewGenerateMessage builds a dispatchable message.
func NewGenerateMessage() queue.Message {
	return queue.Message{
		Queue:          QueueSitemap,
		Type:           MessageTypeGenerate,
		IdempotencyKey: MessageTypeGenerate,
	}
}

// Registration describes the sitemap job.
//
// Serialised globally: two concurrent runs would both render the whole file and
// race on the same object key.
func (generator *Generator) Registration() jobs.Registration {
	return jobs.Registration{
		Type:        MessageTypeGenerate,
		Timeout:     GenerateTimeout,
		MaxAttempts: GenerateMaxAttempts,
		Handler: jobs.HandlerFunc(func(ctx context.Context, _ queue.Message) error {
			_, _, err := generator.Generate(ctx)
			return err
		}),
		SerialKey:  func(queue.Message) (string, error) { return MessageTypeGenerate, nil },
		LaravelJob: "App\\Console\\Commands\\GenerateSitemap",
	}
}
