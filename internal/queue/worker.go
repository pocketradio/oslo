package queue

import (
	"context"
	"fmt"
	"sync"
)

type Handler func(context.Context, Message) error

type Worker struct {
	consumer Consumer
	deleter  Deleter
	handler  Handler
	onError  func(error)
	workers  int
}

// validates queue worker dependencies and creates the worker pool.
// consuming, handling, and deletion are supplied through small interfaces.
func NewWorker(consumer Consumer, deleter Deleter, workers int, handler Handler, onError func(error)) (*Worker, error) {
	if consumer == nil {
		return nil, fmt.Errorf("consumer is required")
	}
	if deleter == nil {
		return nil, fmt.Errorf("deleter is required")
	}
	if workers < 1 {
		return nil, fmt.Errorf("workers must be at least 1")
	}
	if handler == nil {
		return nil, fmt.Errorf("handler is required")
	}
	if onError == nil {
		onError = func(error) {}
	}

	return &Worker{
		consumer: consumer,
		deleter:  deleter,
		handler:  handler,
		onError:  onError,
		workers:  workers,
	}, nil
}

// starts receiver and handler goroutines until shutdown or a receive failure.
// a wait group ensures all in-flight deliveries finish before returning.
func (w *Worker) ServeQueue(ctx context.Context) error {
	jobs := make(chan Delivery, w.workers)
	var wg sync.WaitGroup

	wg.Add(w.workers)
	for range w.workers {
		go w.handleQueueDelivery(ctx, jobs, &wg)
	}

	err := w.forwardSQSDeliveries(ctx, jobs)
	close(jobs)
	wg.Wait()

	return err
}

// handles deliveries from the jobs channel and deletes successful messages.
// failed handling leaves the message undeleted so sqs can retry it.
func (w *Worker) handleQueueDelivery(ctx context.Context, jobs <-chan Delivery, wg *sync.WaitGroup) {
	defer wg.Done()

	// receives values from channel until it's closed

	for delivery := range jobs {
		if err := w.handler(ctx, delivery.Message); err != nil {
			w.onError(fmt.Errorf("handle queue message %s: %w", delivery.Message.ID, err))
			continue
		}
		if err := w.deleter.Delete(ctx, delivery.ReceiptHandle); err != nil {
			w.onError(fmt.Errorf("delete queue message %s: %w", delivery.Message.ID, err))
			continue
		}
	}
}

// receives batches from sqs and forwards each delivery to worker jobs.
// cancellation closes the forwarding path without inventing empty deliveries.
func (w *Worker) forwardSQSDeliveries(ctx context.Context, jobs chan<- Delivery) error {
	for {
		deliveries, err := w.consumer.Receive(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("receive queue messages: %w", err)
		}

		for _, delivery := range deliveries {
			select {
			case jobs <- delivery:
			case <-ctx.Done():
				return nil
			}
		}
	}
}
