package aws_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	omniAws "github.com/mitchelldw01/omnirepo/internal/service/aws"
)

func TestLock(t *testing.T) {
	project, table := "omnirepo", "omnirepo"
	tester, err := newDynamoTester(project, table)
	if err != nil {
		t.Fatal(err)
	}

	locker, err := omniAws.NewAwsLock(tester.client, project, table)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("should create the lock when it doesn't exist", func(t *testing.T) {
		if err := tester.deleteTestLock(); err != nil {
			t.Fatal(err)
		}

		if err := locker.Lock(); err != nil {
			t.Fatal(err)
		}

		isAcquired, err := tester.readTestLock()
		if err != nil {
			t.Fatal(err)
		}

		if isAcquired != true {
			t.Fatalf("expected %v, got %v", true, isAcquired)
		}
	})

	t.Run("should acquire the lock when lock exists and is not acquired", func(t *testing.T) {
		if err := tester.unlockTestLock(); err != nil {
			t.Fatal(err)
		}

		if err := locker.Lock(); err != nil {
			t.Fatal(err)
		}

		isAcquired, err := tester.readTestLock()
		if err != nil {
			t.Fatal(err)
		}

		if isAcquired != true {
			t.Fatalf("expected %v, got %v", true, isAcquired)
		}
	})

	t.Run("should return an error when the lock is already acquired", func(t *testing.T) {
		if err := tester.lockTestLock(); err != nil {
			t.Fatal(err)
		}

		if err := locker.Lock(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestUnlock(t *testing.T) {
	project, table := "omnirepo", "omnirepo"
	tester, err := newDynamoTester(project, table)
	if err != nil {
		t.Fatal(err)
	}

	locker, err := omniAws.NewAwsLock(tester.client, project, table)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("should free the lock when its currently locked", func(t *testing.T) {
		if err := tester.lockTestLock(); err != nil {
			t.Fatal(err)
		}

		if err := locker.Unlock(); err != nil {
			t.Fatal(err)
		}

		isAcquired, err := tester.readTestLock()
		if err != nil {
			t.Fatal(err)
		}

		if isAcquired != false {
			t.Fatalf("expected %v, got %v", false, isAcquired)
		}
	})

	t.Run("should return an error when the lock is already unlocked", func(t *testing.T) {
		if err := tester.unlockTestLock(); err != nil {
			t.Fatal(err)
		}

		if err := locker.Unlock(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

type dynamoEndpointResolver struct{}

func (er *dynamoEndpointResolver) ResolveEndpoint(service, region string) (aws.Endpoint, error) {
	return aws.Endpoint{
		PartitionID:       "aws",
		URL:               "http://localhost:8000",
		SigningRegion:     "us-east-1",
		HostnameImmutable: true,
	}, nil
}

type lockTester struct {
	client  *dynamodb.Client
	project string
	table   string
}

func newDynamoTester(project, table string) (*lockTester, error) {
	client := dynamodb.NewFromConfig(aws.Config{
		Region:           "us-east-1",
		Credentials:      aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider("test", "test", "")),
		EndpointResolver: &dynamoEndpointResolver{},
	})

	if err := createTestTable(client, table); err != nil {
		return nil, err
	}

	return &lockTester{
		client:  client,
		project: project,
		table:   table,
	}, nil
}

func createTestTable(client *dynamodb.Client, table string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: &table,
	})
	if err == nil {
		return nil
	}

	var notFoundErr *types.ResourceNotFoundException
	if ok := errors.As(err, &notFoundErr); ok {
		input := getCreateTableInput(table)
		_, err := client.CreateTable(ctx, &input)
		if err != nil {
			return fmt.Errorf("failed to create test table: %v", err)
		}
		return nil
	}

	return nil
}

func getCreateTableInput(table string) dynamodb.CreateTableInput {
	return dynamodb.CreateTableInput{
		TableName: &table,
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("ProjectName"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("ProjectName"),
				KeyType:       types.KeyTypeHash,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	}
}

func (lt *lockTester) deleteTestLock() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	input := &dynamodb.DeleteItemInput{
		TableName: &lt.table,
		Key: map[string]types.AttributeValue{
			"ProjectName": &types.AttributeValueMemberS{Value: lt.project},
		},
	}

	_, err := lt.client.DeleteItem(ctx, input)
	if err != nil {
		return err
	}

	return nil
}

func (lt *lockTester) readTestLock() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	input := lt.getReadTestLockInput()
	result, err := lt.client.GetItem(ctx, &input)
	if err != nil {
		return false, fmt.Errorf("failed to read test lock: %v", err)
	}
	if result.Item == nil {
		return false, nil
	}

	attributeValue, exists := result.Item["LockAcquired"]
	if !exists {
		return false, nil
	}

	lockAcquired := attributeValue.(*types.AttributeValueMemberBOOL)
	return lockAcquired.Value, nil
}

func (lt *lockTester) getReadTestLockInput() dynamodb.GetItemInput {
	return dynamodb.GetItemInput{
		TableName: &lt.table,
		Key: map[string]types.AttributeValue{
			"ProjectName": &types.AttributeValueMemberS{Value: lt.project},
		},
		AttributesToGet: []string{
			"LockAcquired",
		},
	}
}

func (lt *lockTester) unlockTestLock() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	input := lt.getUnlockTestLockInput()
	if _, err := lt.client.UpdateItem(ctx, &input); err != nil {
		var apiErr *types.ConditionalCheckFailedException
		if ok := errors.As(err, &apiErr); ok {
			return nil
		}

		return fmt.Errorf("failed to release test lock: %v", err)
	}

	return nil
}

func (lt *lockTester) getUnlockTestLockInput() dynamodb.UpdateItemInput {
	return dynamodb.UpdateItemInput{
		TableName: aws.String(lt.table),
		Key: map[string]types.AttributeValue{
			"ProjectName": &types.AttributeValueMemberS{Value: lt.project},
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

func (lt *lockTester) lockTestLock() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	input := lt.getLockTestLockInput()
	if _, err := lt.client.UpdateItem(ctx, &input); err != nil {
		var apiErr *types.ConditionalCheckFailedException
		if ok := errors.As(err, &apiErr); ok {
			return nil
		}

		return fmt.Errorf("failed to acquire test lock: %v", err)
	}

	return nil
}

func (lt *lockTester) getLockTestLockInput() dynamodb.UpdateItemInput {
	return dynamodb.UpdateItemInput{
		TableName: aws.String(lt.table),
		Key: map[string]types.AttributeValue{
			"ProjectName": &types.AttributeValueMemberS{Value: lt.project},
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
