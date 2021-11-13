package k8s

// Label keys that will be used to update the Pods metadata
const (
	LabelPrefix      = "cluster.proto.actor/"
	LabelPort        = LabelPrefix + "port"
	LabelKind        = LabelPrefix + "kind"
	LabelCluster     = LabelPrefix + "cluster"
	LabelStatusValue = LabelPrefix + "status-value"
	LabelMemberID    = LabelPrefix + "member-id"
)
