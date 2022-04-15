package automanaged

// NodeModel represents a node in the cluster
type NodeModel struct {
	ID             string   `json:"id"`
	Address        string   `json:"address"`
	AutoManagePort int      `json:"auto_manage_port"`
	Port           int      `json:"port"`
	Kinds          []string `json:"kinds"`
	ClusterName    string   `json:"cluster_name"`
}

// NewNode returns a new node for the cluster
func NewNode(clusterName string, id string, address string, port int, autoManPort int, kind []string) *NodeModel {
	return &NodeModel{
		ID:             id,
		ClusterName:    clusterName,
		Address:        address,
		Port:           port,
		AutoManagePort: autoManPort,
		Kinds:          kind,
	}
}
