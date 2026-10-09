#!/bin/sh

dlq_url="$(awslocal sqs create-queue --queue-name oslo-matching-dlq --query QueueUrl --output text)"
dlq_arn="$(awslocal sqs get-queue-attributes --queue-url "$dlq_url" --attribute-names QueueArn --query 'Attributes.QueueArn' --output text)"

awslocal sqs create-queue \
  --queue-name oslo-matching \
  --attributes "{\"RedrivePolicy\":\"{\\\"deadLetterTargetArn\\\":\\\"$dlq_arn\\\",\\\"maxReceiveCount\\\":\\\"3\\\"}\"}" \
  >/dev/null
