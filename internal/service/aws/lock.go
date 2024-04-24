package aws

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type AwsLock struct {
	client  *dynamodb.Client
	project string
	table   string
}

func NewAwsLock(client *dynamodb.Client, project, table string) (AwsLock, error) {
	if project == "" {
		return AwsLock{}, errors.New("remote caching requires a project name to be defined")
	}
	if table == "" {
		return AwsLock{}, errors.New("remote caching requires a table name to be defined")
	}

	return AwsLock{
		client:  client,
		project: project,
		table:   table,
	}, nil
}

func (l AwsLock) Lock() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	input := l.createLockInput()
	if _, err := l.client.UpdateItem(ctx, &input); err != nil {
		var apiErr *types.ConditionalCheckFailedException
		if ok := errors.As(err, &apiErr); ok {
			return fmt.Errorf("lock is already acquired... run 'omni unlock' to cancel")
		}

		return fmt.Errorf("failed to acquire cache lock: %v", err)
	}

	return nil
}

func (l AwsLock) createLockInput() dynamodb.UpdateItemInput {
	return dynamodb.UpdateItemInput{
		TableName: aws.String(l.table),
		Key: map[string]types.AttributeValue{
			"ProjectName": &types.AttributeValueMemberS{Value: l.project},
		},
		UpdateExpression: aws.String("SET LockAcquired = :newval"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":newval":     &types.AttributeValueMemberBOOL{Value: true},
			":currentval": &types.AttributeValueMemberBOOL{Value: false},
		},
		ConditionExpression: aws.String("attribute_not_exists(LockAcquired) OR LockAcquired = :currentval"),
		ReturnValues:        types.ReturnValueUpdatedNew,
	}
}

func (l AwsLock) Unlock() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	input := l.createUnlockInput()
	if _, err := l.client.UpdateItem(ctx, &input); err != nil {
		return fmt.Errorf("failed to release cache lock: %v", err)
	}

	return nil
}

func (l AwsLock) createUnlockInput() dynamodb.UpdateItemInput {
	return dynamodb.UpdateItemInput{
		TableName: aws.String(l.table),
		Key: map[string]types.AttributeValue{
			"ProjectName": &types.AttributeValueMemberS{Value: l.project},
		},
		UpdateExpression: aws.String("SET LockAcquired = :newval"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":newval": &types.AttributeValueMemberBOOL{Value: false},
		},
		ConditionExpression: aws.String("LockAcquired = :currentval"),
		ReturnValues:        types.ReturnValueUpdatedNew,
	}
}
