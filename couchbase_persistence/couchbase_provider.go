package couchbase_persistence

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"

	"github.com/AsynkronIT/gam/persistence"
	"github.com/couchbase/gocb"
	proto "github.com/golang/protobuf/proto"
)

type Provider struct {
	*persistence.NoSnapshotSupport
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

func formatKey(actorName string, eventIndex int) string {
	key := fmt.Sprintf("%v-%010d", actorName, eventIndex)
	return key
}

func (provider *Provider) GetEvents(actorName string, callback func(event interface{})) {
	q := gocb.NewN1qlQuery("SELECT b.* FROM `" + provider.bucketName + "` b WHERE meta(b).id >= $1 and meta(b).id <= $2")
	var p []interface{}
	p = append(p, formatKey(actorName, 0))
	p = append(p, formatKey(actorName, 9999999999))

	rows, err := provider.bucket.ExecuteN1qlQuery(q, p)
	if err != nil {
		log.Fatalf("Error executing N1ql: %v", err)
	}
	defer rows.Close()
	var row Envelope

	for rows.Next(&row) {
		e := unpackMessage(row)
		log.Printf("%+v\n", e)
		callback(e)
	}
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
		Type:    typeName,
		Message: bytes,
	}
	key := formatKey(actorName, eventIndex)
	_, err = provider.bucket.Insert(key, envelope, 0)
	if err != nil {
		log.Fatal(err)
	}
}

type Envelope struct {
	Type    string          `json:"type"`
	Message json.RawMessage `json:"event"`
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
