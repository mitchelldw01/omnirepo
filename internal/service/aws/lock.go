package aws

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func NewDynamoClient(workspace, region string) (*dynamodb.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %v", err)
	}

	if region != "" {
		cfg.Region = region
	}

	return dynamodb.NewFromConfig(cfg), nil
}

type AwsLock struct {
	client    *dynamodb.Client
	workspace string
	table     string
}

func NewAwsLock(client *dynamodb.Client, workspace, table string) *AwsLock {
	return &AwsLock{
		client:    client,
		workspace: workspace,
		table:     table,
	}
}

func (l *AwsLock) Lock() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	input := l.getLockInput()
	if _, err := l.client.UpdateItem(ctx, &input); err != nil {
		var apiErr *types.ConditionalCheckFailedException
		if ok := errors.As(err, &apiErr); ok {
			return fmt.Errorf("lock is already acquired... run 'omni unlock' to cancel")
		}

		return fmt.Errorf("failed to acquire cache lock: %v", err)
	}

	return nil
}

func (l *AwsLock) getLockInput() dynamodb.UpdateItemInput {
	// Sets the value of `LockAcquired` to `true` for the item with the given `WorkspaceName`.
	// If the item does not exist, it will be created.
	return dynamodb.UpdateItemInput{
		TableName: aws.String(l.table),
		Key: map[string]types.AttributeValue{
			"WorkspaceName": &types.AttributeValueMemberS{Value: l.workspace},
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

func (l *AwsLock) Unlock() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	input := l.getUnlockInput()
	if _, err := l.client.UpdateItem(ctx, &input); err != nil {
		return fmt.Errorf("failed to release cache lock: %v", err)
	}

	return nil
}

func (l *AwsLock) getUnlockInput() dynamodb.UpdateItemInput {
	// Sets `LockAcquired` to `false` on the item with the given `WorkspaceName`.
	return dynamodb.UpdateItemInput{
		TableName: aws.String(l.table),
		Key: map[string]types.AttributeValue{
			"WorkspaceName": &types.AttributeValueMemberS{Value: l.workspace},
		},
		UpdateExpression: aws.String("SET LockAcquired = :newval"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":newval":     &types.AttributeValueMemberBOOL{Value: false},
			":currentval": &types.AttributeValueMemberBOOL{Value: true},
		},
		ConditionExpression: aws.String("LockAcquired = :currentval"),
		ReturnValues:        types.ReturnValueUpdatedNew,
	}
}
