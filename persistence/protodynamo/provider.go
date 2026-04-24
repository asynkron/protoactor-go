package protodynamo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/awevoke/protoactor-go/persistence"
)

const (
	// maxBatchWrite is the DynamoDB limit for BatchWriteItem requests.
	maxBatchWrite = 25

	// maxEventIndex is used as the upper bound when eventIndexEnd is 0 (meaning "all").
	maxEventIndex = 999999999
)

// Compile-time check that DynamoDBProvider satisfies persistence.ProviderState.
var _ persistence.ProviderState = (*DynamoDBProvider)(nil)

// DynamoDBProvider implements persistence.ProviderState using AWS DynamoDB.
type DynamoDBProvider struct {
	client *dynamodb.Client
	config *Config
}

// New creates a new DynamoDBProvider with the given options. At minimum,
// WithClient must be provided.
func New(opts ...Option) (*DynamoDBProvider, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}

	if cfg.Client == nil {
		return nil, errors.New("protodynamo: DynamoDB client is required; use WithClient option")
	}

	return &DynamoDBProvider{
		client: cfg.Client,
		config: cfg,
	}, nil
}

// Restart is a no-op for DynamoDB; the data is durable in the service.
func (p *DynamoDBProvider) Restart() {}

// GetSnapshotInterval returns the configured snapshot interval.
func (p *DynamoDBProvider) GetSnapshotInterval() int {
	return p.config.SnapshotInterval
}

// GetEvents queries the events table for the given actor and invokes callback
// for each event in ascending order by EventIndex. When eventIndexEnd is 0 it
// means "return all events from eventIndexStart onward".
func (p *DynamoDBProvider) GetEvents(actorName string, eventIndexStart int, eventIndexEnd int, callback func(e any)) {
	ctx := context.Background()

	endIndex := eventIndexEnd
	if endIndex == 0 {
		endIndex = maxEventIndex
	}

	// Build key condition: ActorName = :name AND EventIndex BETWEEN :start AND :end
	// The Go persistence interface treats eventIndexEnd as exclusive (like a
	// slice), but BETWEEN is inclusive on both sides, so subtract 1 from the
	// upper bound.
	input := &dynamodb.QueryInput{
		TableName:              aws.String(p.config.EventsTable),
		ConsistentRead:         aws.Bool(true),
		ScanIndexForward:       aws.Bool(true),
		KeyConditionExpression: aws.String("ActorName = :name AND EventIndex BETWEEN :start AND :endidx"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":name":   &types.AttributeValueMemberS{Value: actorName},
			":start":  &types.AttributeValueMemberN{Value: strconv.Itoa(eventIndexStart)},
			":endidx": &types.AttributeValueMemberN{Value: strconv.Itoa(endIndex - 1)},
		},
	}

	paginator := dynamodb.NewQueryPaginator(p.client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			slog.Error("protodynamo: GetEvents query failed",
				"actor", actorName,
				"error", err,
			)
			return
		}

		for _, item := range page.Items {
			msg, err := p.unmarshalItem(item)
			if err != nil {
				slog.Error("protodynamo: GetEvents unmarshal failed",
					"actor", actorName,
					"error", err,
				)
				continue
			}
			callback(msg)
		}
	}
}

// PersistEvent stores a single event in the events table.
func (p *DynamoDBProvider) PersistEvent(actorName string, eventIndex int, event proto.Message) {
	ctx := context.Background()

	item, err := p.marshalItem(actorName, "EventIndex", eventIndex, event)
	if err != nil {
		slog.Error("protodynamo: PersistEvent marshal failed",
			"actor", actorName,
			"eventIndex", eventIndex,
			"error", err,
		)
		return
	}

	_, err = p.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(p.config.EventsTable),
		Item:      item,
	})
	if err != nil {
		slog.Error("protodynamo: PersistEvent PutItem failed",
			"actor", actorName,
			"eventIndex", eventIndex,
			"error", err,
		)
	}
}

// DeleteEvents deletes all events for the given actor with EventIndex <= inclusiveToIndex.
func (p *DynamoDBProvider) DeleteEvents(actorName string, inclusiveToIndex int) {
	ctx := context.Background()

	// Query to find all event keys up to inclusiveToIndex.
	keys, err := p.queryKeys(ctx, p.config.EventsTable, "ActorName", "EventIndex", actorName, inclusiveToIndex)
	if err != nil {
		slog.Error("protodynamo: DeleteEvents query failed",
			"actor", actorName,
			"error", err,
		)
		return
	}

	p.batchDelete(ctx, p.config.EventsTable, keys)
}

// GetSnapshot retrieves the latest snapshot for the given actor. It queries
// the snapshots table in reverse sort-key order with Limit=1.
func (p *DynamoDBProvider) GetSnapshot(actorName string) (snapshot any, eventIndex int, ok bool) {
	ctx := context.Background()

	input := &dynamodb.QueryInput{
		TableName:              aws.String(p.config.SnapshotsTable),
		ConsistentRead:         aws.Bool(true),
		ScanIndexForward:       aws.Bool(false), // descending – latest first
		Limit:                  aws.Int32(1),
		KeyConditionExpression: aws.String("ActorName = :name"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":name": &types.AttributeValueMemberS{Value: actorName},
		},
	}

	result, err := p.client.Query(ctx, input)
	if err != nil {
		slog.Error("protodynamo: GetSnapshot query failed",
			"actor", actorName,
			"error", err,
		)
		return nil, 0, false
	}

	if len(result.Items) == 0 {
		return nil, 0, false
	}

	item := result.Items[0]

	msg, err := p.unmarshalItem(item)
	if err != nil {
		slog.Error("protodynamo: GetSnapshot unmarshal failed",
			"actor", actorName,
			"error", err,
		)
		return nil, 0, false
	}

	idxAttr, exists := item["SnapshotIndex"]
	if !exists {
		slog.Error("protodynamo: GetSnapshot missing SnapshotIndex",
			"actor", actorName,
		)
		return nil, 0, false
	}

	idxVal, numOk := idxAttr.(*types.AttributeValueMemberN)
	if !numOk {
		slog.Error("protodynamo: GetSnapshot SnapshotIndex is not a number",
			"actor", actorName,
		)
		return nil, 0, false
	}

	idx, err := strconv.Atoi(idxVal.Value)
	if err != nil {
		slog.Error("protodynamo: GetSnapshot SnapshotIndex parse failed",
			"actor", actorName,
			"error", err,
		)
		return nil, 0, false
	}

	return msg, idx, true
}

// PersistSnapshot stores a snapshot in the snapshots table.
func (p *DynamoDBProvider) PersistSnapshot(actorName string, snapshotIndex int, snapshot proto.Message) {
	ctx := context.Background()

	item, err := p.marshalItem(actorName, "SnapshotIndex", snapshotIndex, snapshot)
	if err != nil {
		slog.Error("protodynamo: PersistSnapshot marshal failed",
			"actor", actorName,
			"snapshotIndex", snapshotIndex,
			"error", err,
		)
		return
	}

	_, err = p.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(p.config.SnapshotsTable),
		Item:      item,
	})
	if err != nil {
		slog.Error("protodynamo: PersistSnapshot PutItem failed",
			"actor", actorName,
			"snapshotIndex", snapshotIndex,
			"error", err,
		)
	}
}

// DeleteSnapshots deletes all snapshots for the given actor with SnapshotIndex <= inclusiveToIndex.
func (p *DynamoDBProvider) DeleteSnapshots(actorName string, inclusiveToIndex int) {
	ctx := context.Background()

	keys, err := p.queryKeys(ctx, p.config.SnapshotsTable, "ActorName", "SnapshotIndex", actorName, inclusiveToIndex)
	if err != nil {
		slog.Error("protodynamo: DeleteSnapshots query failed",
			"actor", actorName,
			"error", err,
		)
		return
	}

	p.batchDelete(ctx, p.config.SnapshotsTable, keys)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// marshalItem builds a DynamoDB item map from a protobuf message.
func (p *DynamoDBProvider) marshalItem(actorName, sortKeyName string, sortKeyValue int, msg proto.Message) (map[string]types.AttributeValue, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("proto.Marshal: %w", err)
	}

	typeName := string(proto.MessageName(msg))
	if typeName == "" {
		return nil, fmt.Errorf("proto.MessageName returned empty for %T", msg)
	}

	return map[string]types.AttributeValue{
		"ActorName": &types.AttributeValueMemberS{Value: actorName},
		sortKeyName: &types.AttributeValueMemberN{Value: strconv.Itoa(sortKeyValue)},
		"Data":      &types.AttributeValueMemberB{Value: data},
		"DataType":  &types.AttributeValueMemberS{Value: typeName},
	}, nil
}

// unmarshalItem deserializes a DynamoDB item back into a protobuf message.
func (p *DynamoDBProvider) unmarshalItem(item map[string]types.AttributeValue) (proto.Message, error) {
	dataTypeAttr, ok := item["DataType"]
	if !ok {
		return nil, errors.New("item missing DataType attribute")
	}

	dataTypeMember, ok := dataTypeAttr.(*types.AttributeValueMemberS)
	if !ok {
		return nil, errors.New("DataType attribute is not a string")
	}

	dataAttr, ok := item["Data"]
	if !ok {
		return nil, errors.New("item missing Data attribute")
	}

	dataMember, ok := dataAttr.(*types.AttributeValueMemberB)
	if !ok {
		return nil, errors.New("Data attribute is not binary")
	}

	// Look up the message type in the global protobuf registry.
	fullName := protoreflect.FullName(dataTypeMember.Value)

	msgType, err := protoregistry.GlobalTypes.FindMessageByName(fullName)
	if err != nil {
		return nil, fmt.Errorf("protoregistry lookup for %q: %w", fullName, err)
	}

	msg := msgType.New().Interface()
	if err := proto.Unmarshal(dataMember.Value, msg); err != nil {
		return nil, fmt.Errorf("proto.Unmarshal for %q: %w", fullName, err)
	}

	return msg, nil
}

// queryKeys queries a table for all items matching the given actor name with
// sort key <= inclusiveToIndex. It returns only the key attributes for each item.
func (p *DynamoDBProvider) queryKeys(ctx context.Context, tableName, hashKeyName, sortKeyName, actorName string, inclusiveToIndex int) ([]map[string]types.AttributeValue, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		ConsistentRead:         aws.Bool(true),
		ProjectionExpression:   aws.String(hashKeyName + ", " + sortKeyName),
		KeyConditionExpression: aws.String(hashKeyName + " = :name AND " + sortKeyName + " <= :idx"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":name": &types.AttributeValueMemberS{Value: actorName},
			":idx":  &types.AttributeValueMemberN{Value: strconv.Itoa(inclusiveToIndex)},
		},
	}

	var keys []map[string]types.AttributeValue

	paginator := dynamodb.NewQueryPaginator(p.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range page.Items {
			keys = append(keys, map[string]types.AttributeValue{
				hashKeyName: item[hashKeyName],
				sortKeyName: item[sortKeyName],
			})
		}
	}

	return keys, nil
}

// batchDelete deletes items by key in batches of up to 25.
func (p *DynamoDBProvider) batchDelete(ctx context.Context, tableName string, keys []map[string]types.AttributeValue) {
	for i := 0; i < len(keys); i += maxBatchWrite {
		end := i + maxBatchWrite
		if end > len(keys) {
			end = len(keys)
		}

		var requests []types.WriteRequest
		for _, key := range keys[i:end] {
			requests = append(requests, types.WriteRequest{
				DeleteRequest: &types.DeleteRequest{Key: key},
			})
		}

		_, err := p.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				tableName: requests,
			},
		})
		if err != nil {
			slog.Error("protodynamo: batchDelete failed",
				"table", tableName,
				"batchStart", i,
				"error", err,
			)
			return
		}
	}
}
