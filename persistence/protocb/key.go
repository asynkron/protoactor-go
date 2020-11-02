package protocb

import "fmt"

func formatEventKey(actorName string, eventIndex int) string {
	key := fmt.Sprintf("%v-event-%010d", actorName, eventIndex)
	return key
}

func formatSnapshotKey(actorName string, eventIndex int) string {
	key := fmt.Sprintf("%v-snapshot-%010d", actorName, eventIndex)
	return key
}
