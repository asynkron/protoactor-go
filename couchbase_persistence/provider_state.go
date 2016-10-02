package couchbase_persistence

import (
	"log"

	"github.com/couchbase/gocb"
	proto "github.com/golang/protobuf/proto"
)

type ProviderState struct {
	*Provider
}

func (provider *ProviderState) GetEvents(actorName string, eventIndexStart int, callback func(event interface{})) {
	q := gocb.NewN1qlQuery("SELECT b.* FROM `" + provider.bucketName + "` b WHERE meta(b).id >= $1 and meta(b).id <= $2")
	var p []interface{}
	p = append(p, formatEventKey(actorName, eventIndexStart))
	p = append(p, formatEventKey(actorName, 9999999999))

	rows, err := provider.bucket.ExecuteN1qlQuery(q, p)
	if err != nil {
		log.Fatalf("Error executing N1ql: %v", err)
	}
	defer rows.Close()
	var row envelope

	for rows.Next(&row) {
		e := row.message()
		callback(e)
	}
}

func (provider *ProviderState) GetSnapshot(actorName string) (snapshot interface{}, eventIndex int, ok bool) {
	q := gocb.NewN1qlQuery("SELECT b.* FROM `" + provider.bucketName + "` b WHERE meta(b).id >= $1 and meta(b).id <= $2 order by b.eventIndex desc limit 1")
	var p []interface{}
	p = append(p, formatSnapshotKey(actorName, 0))
	p = append(p, formatSnapshotKey(actorName, 9999999999))

	rows, err := provider.bucket.ExecuteN1qlQuery(q, p)
	if err != nil {
		log.Fatalf("Error executing N1ql: %v", err)
	}
	defer rows.Close()
	var row envelope
	if rows.Next(&row) {
		return row.message(), row.EventIndex, true
	}
	return nil, 0, false
}
func (provider *Provider) GetSnapshotInterval() int {
	return provider.snapshotInterval
}

func (provider *ProviderState) PersistEvent(actorName string, eventIndex int, event proto.Message) {
	key := formatEventKey(actorName, eventIndex)
	envelope := newEnvelope(event, "event", eventIndex)
	persist := func() {
		_, err := provider.bucket.Insert(key, envelope, 0)
		if err != nil {
			log.Fatal(err)
		}
	}
	if provider.async {
		provider.writer.Tell(&write{fun: persist})
	} else {
		persist()
	}
}

func (provider *ProviderState) PersistSnapshot(actorName string, eventIndex int, snapshot proto.Message) {
	key := formatSnapshotKey(actorName, eventIndex)
	envelope := newEnvelope(snapshot, "snapshot", eventIndex)
	persist := func() {
		_, err := provider.bucket.Insert(key, envelope, 0)
		if err != nil {
			log.Fatal(err)
		}
	}

	if provider.async {
		provider.writer.Tell(&write{fun: persist})
	} else {
		persist()
	}
}
