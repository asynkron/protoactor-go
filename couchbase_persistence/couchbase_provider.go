package couchbase_persistence

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/AsynkronIT/gam/persistence"
	"github.com/couchbase/gocb"
	proto "github.com/golang/protobuf/proto"
)

type Provider struct {
	*persistence.NoSnapshotSupport
	bucket *gocb.Bucket
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
		bucket: bucket,
	}
}

func (provider *Provider) GetEvents(actorName string) []proto.Message {
	var myValue interface{}
	provider.bucket.Get("1-3", &myValue)
	q := gocb.NewN1qlQuery("SELECT b.* FROM `labb` b WHERE meta(b).id >= \"1-0000000000\"")
	r, err := provider.bucket.ExecuteN1qlQuery(q, nil)
	if err != nil {
		log.Fatalf("Error executing N1ql: %v", err)
	}
	log.Println(r)

	log.Printf("%+v\n", myValue)
	return nil
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
	key := fmt.Sprintf("%v-%010d", actorName, eventIndex)
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
