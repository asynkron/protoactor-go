package protodynamo

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// CreateTablesIfNotExist creates the events and snapshots DynamoDB tables if
// they do not already exist. It uses on-demand (PAY_PER_REQUEST) billing mode.
//
// The events table has:
//   - ActorName (S) as the partition key
//   - EventIndex (N) as the sort key
//
// The snapshots table has:
//   - ActorName (S) as the partition key
//   - SnapshotIndex (N) as the sort key
func (p *DynamoDBProvider) CreateTablesIfNotExist(ctx context.Context) error {
	if err := p.createTableIfNotExist(ctx, p.config.EventsTable, "ActorName", "EventIndex"); err != nil {
		return fmt.Errorf("create events table %q: %w", p.config.EventsTable, err)
	}

	if err := p.createTableIfNotExist(ctx, p.config.SnapshotsTable, "ActorName", "SnapshotIndex"); err != nil {
		return fmt.Errorf("create snapshots table %q: %w", p.config.SnapshotsTable, err)
	}

	return nil
}

func (p *DynamoDBProvider) createTableIfNotExist(ctx context.Context, tableName, hashKey, rangeKey string) error {
	_, err := p.client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String(hashKey),
				KeyType:       types.KeyTypeHash,
			},
			{
				AttributeName: aws.String(rangeKey),
				KeyType:       types.KeyTypeRange,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String(hashKey),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String(rangeKey),
				AttributeType: types.ScalarAttributeTypeN,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		// If the table already exists, that is not an error.
		var resourceInUse *types.ResourceInUseException
		if errors.As(err, &resourceInUse) {
			return nil
		}

		return err
	}

	return nil
}
