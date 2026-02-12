package protocb

import (
	"log/slog"
	"sync"

	"github.com/couchbase/gocb"
	"google.golang.org/protobuf/proto"
)

type cbState struct {
	*Provider
	wg sync.WaitGroup
}

func (state *cbState) Restart() {
	// wait for any pending writes to complete
	state.wg.Wait()
}

func (state *cbState) GetEvents(actorName string, eventIndexStart int, eventIndexEnd int, callback func(event interface{})) {
	q := gocb.NewN1qlQuery("SELECT b.* FROM `" + state.bucketName + "` b WHERE meta(b).id >= $1 and meta(b).id <= $2")
	q.Consistency(gocb.RequestPlus)

	// read all
	if eventIndexEnd == 0 {
		eventIndexEnd = 9999999999
	}

	var p []interface{}
	p = append(p, formatEventKey(actorName, eventIndexStart))
	p = append(p, formatEventKey(actorName, eventIndexEnd))

	rows, err := state.bucket.ExecuteN1qlQuery(q, p)
	if err != nil {
		slog.Error("Failed to execute N1QL query for GetEvents", "actor", actorName, "error", err)
		return
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			slog.Error("Failed to close N1QL result rows", "actor", actorName, "error", err)
		}
	}()

	var row envelope
	i := eventIndexStart
	for rows.Next(&row) {
		e := row.message()
		if row.EventIndex != i {
			slog.Error("Invalid actor state, missing event", "actor", actorName, "expectedIndex", i)
			return
		}
		callback(e)
		i++
	}
}

func (state *cbState) GetSnapshot(actorName string) (snapshot interface{}, eventIndex int, ok bool) {
	q := gocb.NewN1qlQuery("SELECT b.* FROM `" + state.bucketName + "` b WHERE meta(b).id >= $1 and meta(b).id <= $2 order by b.eventIndex desc limit 1")
	q.Consistency(gocb.RequestPlus)

	var p []interface{}
	p = append(p, formatSnapshotKey(actorName, 0))
	p = append(p, formatSnapshotKey(actorName, 9999999999))

	rows, err := state.bucket.ExecuteN1qlQuery(q, p)
	if err != nil {
		slog.Error("Failed to execute N1QL query for GetSnapshot", "actor", actorName, "error", err)
		return nil, 0, false
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			slog.Error("Failed to close N1QL result rows", "actor", actorName, "error", err)
		}
	}()

	var row envelope
	if rows.Next(&row) {
		return row.message(), row.EventIndex, true
	}
	return nil, 0, false
}

func (provider *Provider) GetSnapshotInterval() int {
	return provider.snapshotInterval
}

func (state *cbState) PersistEvent(actorName string, eventIndex int, event proto.Message) {
	key := formatEventKey(actorName, eventIndex)
	envelope := newEnvelope(event, "event", eventIndex)
	state.persistEnvelope(key, envelope)
}

func (state *cbState) DeleteEvents(actorName string, inclusiveToIndex int) {
	q := gocb.NewN1qlQuery("DELETE FROM `" + state.bucketName + "` b WHERE meta(b).id >= $1 AND meta(b).id <= $2")
	var p []interface{}
	p = append(p, formatEventKey(actorName, 0))
	p = append(p, formatEventKey(actorName, inclusiveToIndex))
	_, err := state.bucket.ExecuteN1qlQuery(q, p)
	if err != nil {
		slog.Error("Failed to delete events", "actor", actorName, "toIndex", inclusiveToIndex, "error", err)
	}
}

func (state *cbState) PersistSnapshot(actorName string, eventIndex int, snapshot proto.Message) {
	key := formatSnapshotKey(actorName, eventIndex)
	envelope := newEnvelope(snapshot, "snapshot", eventIndex)
	state.persistEnvelope(key, envelope)
}

func (state *cbState) DeleteSnapshots(actorName string, inclusiveToIndex int) {
	q := gocb.NewN1qlQuery("DELETE FROM `" + state.bucketName + "` b WHERE meta(b).id >= $1 AND meta(b).id <= $2")
	var p []interface{}
	p = append(p, formatSnapshotKey(actorName, 0))
	p = append(p, formatSnapshotKey(actorName, inclusiveToIndex))
	_, err := state.bucket.ExecuteN1qlQuery(q, p)
	if err != nil {
		slog.Error("Failed to delete snapshots", "actor", actorName, "toIndex", inclusiveToIndex, "error", err)
	}
}

func (state *cbState) persistEnvelope(key string, envelope *envelope) {
	state.wg.Add(1)
	persist := func() {
		_, err := state.bucket.Insert(key, envelope, 0)
		if err != nil {
			slog.Error("Failed to persist envelope", "key", key, "error", err)
		}
		state.wg.Done()
	}
	if state.async {
		state.actorSystem.Root.Send(state.writer, &write{fun: persist})
	} else {
		persist()
	}
}
