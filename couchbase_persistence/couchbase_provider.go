package couchbase_persistence

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/AsynkronIT/gam/persistence"
	"github.com/couchbase/gocb"
	"github.com/gogo/protobuf/proto"
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

	log.Printf("%+v\n", myValue)
	return nil
}

func (provider *Provider) PersistEvent(actorName string, eventIndex int, event proto.Message) {
	key := fmt.Sprintf("%v-%v", actorName, eventIndex)
	_, err := provider.bucket.Insert(key, event, 0)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("writing")
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
