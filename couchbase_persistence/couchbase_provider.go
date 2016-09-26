package couchbase_persistence

import (
	"log"

	"github.com/AsynkronIT/gam/persistence"
	"github.com/couchbase/go-couchbase"
)

type Provider struct {
	*persistence.NoSnapshotSupport
}

func New(baseU string, bucketName string) *Provider {
	c, err := couchbase.Connect(baseU)
	if err != nil {
		log.Fatalf("Error connecting:  %v", err)
	}

	pool, err := c.GetPool("default")
	if err != nil {
		log.Fatalf("Error getting pool:  %v", err)
	}

	bucket, err := pool.GetBucket(bucketName)
	if err != nil {
		log.Fatalf("Error getting bucket:  %v", err)
	}

	err = bucket.Set("someKey", 0, []string{"an", "example", "list"})
	if err != nil {
		log.Fatal(err)
	}

	return &Provider{}
}

func (provider *Provider) GetEvents(actorName string) []persistence.PersistentMessage {
	return nil
}

func (provider *Provider) PersistEvent(actorName string, event persistence.PersistentMessage) {
}
