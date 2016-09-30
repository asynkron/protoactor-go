package couchbase_persistence

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"

	"github.com/couchbase/gocb"
	proto "github.com/golang/protobuf/proto"
)

type Provider struct {
	bucket     *gocb.Bucket
	bucketName string
}

func New(bucketName string, baseU string) *Provider {
	c, err := gocb.Connect(baseU)
	if err != nil {
		log.Fatalf("Error connecting:  %v", err)
	}
	bucket, err := c.OpenBucket(bucketName, "")
	if err != nil {
		log.Fatalf("Error getting bucket:  %v", err)
	}
	bucket.SetTranscoder(Transcoder{})

	return &Provider{
		bucket:     bucket,
		bucketName: bucketName,
	}
}

func formatEventKey(actorName string, eventIndex int) string {
	key := fmt.Sprintf("%v-event-%010d", actorName, eventIndex)
	return key
}

func formatSnapshotKey(actorName string, eventIndex int) string {
	key := fmt.Sprintf("%v-snapshot-%010d", actorName, eventIndex)
	return key
}

func (provider *Provider) GetEvents(actorName string, eventIndexStart int, callback func(event interface{})) {
	q := gocb.NewN1qlQuery("SELECT b.* FROM `" + provider.bucketName + "` b WHERE meta(b).id >= $1 and meta(b).id <= $2 and snapshot=false")
	var p []interface{}
	p = append(p, formatEventKey(actorName, eventIndexStart))
	p = append(p, formatEventKey(actorName, 9999999999))

	rows, err := provider.bucket.ExecuteN1qlQuery(q, p)
	if err != nil {
		log.Fatalf("Error executing N1ql: %v", err)
	}
	defer rows.Close()
	var row Envelope

	for rows.Next(&row) {
		e := unpackMessage(row)
		callback(e)
	}
}

func (provider *Provider) GetSnapshot(actorName string) (snapshot interface{}, eventIndex int, ok bool) {
	q := gocb.NewN1qlQuery("SELECT b.* FROM `" + provider.bucketName + "` b WHERE meta(b).id >= $1 and meta(b).id <= $2 and snapshot=true order by b.eventIndex desc limit 1")
	var p []interface{}
	p = append(p, formatSnapshotKey(actorName, 0))
	p = append(p, formatSnapshotKey(actorName, 9999999999))

	rows, err := provider.bucket.ExecuteN1qlQuery(q, p)
	if err != nil {
		log.Fatalf("Error executing N1ql: %v", err)
	}
	defer rows.Close()
	var row Envelope
	if rows.Next(&row) {
		return unpackMessage(row), row.EventIndex, true
	}
	return nil, 0, false
}
func (provider *Provider) GetSnapshotInterval() int {
	return 3
}

func unpackMessage(message Envelope) proto.Message {
	t := proto.MessageType(message.Type).Elem()
	intPtr := reflect.New(t)
	instance := intPtr.Interface().(proto.Message)
	json.Unmarshal(message.Message, instance)
	return instance
}

func (provider *Provider) PersistEvent(actorName string, eventIndex int, event proto.Message) {
	typeName := proto.MessageName(event)
	bytes, err := json.Marshal(event)
	if err != nil {
		log.Fatal(err)
	}
	envelope := &Envelope{
		Type:       typeName,
		Message:    bytes,
		EventIndex: eventIndex,
		Snapshot:   false,
	}
	key := formatEventKey(actorName, eventIndex)
	_, err = provider.bucket.Insert(key, envelope, 0)
	if err != nil {
		log.Println(key)
		log.Fatal(err)
	}
}

func (provider *Provider) PersistSnapshot(actorName string, eventIndex int, snapshot proto.Message) {
	typeName := proto.MessageName(snapshot)
	bytes, err := json.Marshal(snapshot)
	if err != nil {
		log.Fatal(err)
	}
	envelope := &Envelope{
		Type:       typeName,
		Message:    bytes,
		EventIndex: eventIndex,
		Snapshot:   true,
	}
	key := formatSnapshotKey(actorName, eventIndex)
	_, err = provider.bucket.Insert(key, envelope, 0)
	if err != nil {
		log.Fatal(err)
	}
}

type Envelope struct {
	Type       string          `json:"type"`
	Message    json.RawMessage `json:"event"`
	EventIndex int             `json:"eventIndex"`
	Snapshot   bool            `json:"snapshot"`
}

type Transcoder struct {
}

func (t Transcoder) Decode(bytes []byte, flags uint32, out interface{}) error {
	err := json.Unmarshal(bytes, &out)
	if err != nil {
		return err
	}
	return nil
}

func (t Transcoder) Encode(value interface{}) ([]byte, uint32, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return nil, 0, err
	}
	return bytes, 0, nil
}
