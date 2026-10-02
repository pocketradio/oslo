package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type SQSQueue struct {
	client            *sqs.Client
	queueURL          string
	waitTimeSeconds   int32
	visibilityTimeout int32
}

func NewSQSQueue(ctx context.Context, endpoint, region, queueURL string) (*SQSQueue, error) {

	//loading aws settings
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	//nfg returns a new client
	client := sqs.NewFromConfig(cfg, func(options *sqs.Options) {
		if endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
		}
	})

	return &SQSQueue{
		client:            client,
		queueURL:          queueURL,
		waitTimeSeconds:   10,
		visibilityTimeout: 30,
	}, nil
}

func (q *SQSQueue) Publish(ctx context.Context, message Message) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal queue message: %w", err)
	}

	_, err = q.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(q.queueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		return fmt.Errorf("send queue message: %w", err)
	}

	return nil
}

func (q *SQSQueue) Receive(ctx context.Context) ([]Delivery, error) {
	result, err := q.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(q.queueURL),
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     q.waitTimeSeconds,
		VisibilityTimeout:   q.visibilityTimeout,
		// worker will receive msg and it becomes invis for 30s.
		MessageAttributeNames: []string{"All"},
	})
	if err != nil {
		return nil, fmt.Errorf("receive queue messages: %w", err)
	}

	deliveries := make([]Delivery, 0, len(result.Messages))
	for _, message := range result.Messages {
		var decoded Message
		if err := json.Unmarshal([]byte(aws.ToString(message.Body)), &decoded); err != nil {
			return nil, fmt.Errorf("decode queue message: %w", err)
		}

		deliveries = append(deliveries, Delivery{
			Message:       decoded,
			ReceiptHandle: aws.ToString(message.ReceiptHandle),
		})
	}

	return deliveries, nil
}

func (q *SQSQueue) Delete(ctx context.Context, receiptHandle string) error {
	_, err := q.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	if err != nil {
		return fmt.Errorf("delete queue message: %w", err)
	}

	return nil
}
