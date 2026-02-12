package protocb

import (
	"fmt"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/persistence"
	"github.com/couchbase/gocb"
)

type Provider struct {
	async            bool
	bucket           *gocb.Bucket
	bucketName       string
	snapshotInterval int
	writer           *actor.PID
}

func (provider *Provider) GetState() persistence.ProviderState {
	return &cbState{
		Provider: provider,
	}
}

func New(actorSystem *actor.ActorSystem, bucketName string, baseU string, options ...CouchbaseOption) *Provider {
	c, err := gocb.Connect(baseU)
	if err != nil {
		panic(fmt.Sprintf("Error connecting:  %v", err))
	}
	bucket, err := c.OpenBucketWithMt(bucketName, "")
	if err != nil {
		panic(fmt.Sprintf("Error getting bucket:  %v", err))
	}
	bucket.SetTranscoder(transcoder{})

	config := &couchbaseConfig{}
	for _, option := range options {
		option(config)
	}

	provider := &Provider{
		snapshotInterval: config.snapshotInterval,
		async:            config.async,
		bucket:           bucket,
		bucketName:       bucketName,
	}

	if config.async {
		pid := actorSystem.Root.Spawn(actor.PropsFromFunc(newWriter(time.Second / 10000)))
		provider.writer = pid
	}

	return provider
}
