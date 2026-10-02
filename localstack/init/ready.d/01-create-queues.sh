#!/bin/sh

awslocal sqs create-queue --queue-name oslo-matching >/dev/null
# to create the local matching Q automatically when localstack starts