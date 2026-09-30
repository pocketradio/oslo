package queue

import "context"

type Delivery struct {
	Message       Message
	ReceiptHandle string
}

type Publisher interface {
	Publish(ctx context.Context, message Message) error
}

type Consumer interface {
	Receive(ctx context.Context) ([]Delivery, error)
}

type Deleter interface {
	Delete(ctx context.Context, receiptHandle string) error
}
