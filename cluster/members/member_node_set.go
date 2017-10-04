package members

type MemberNode struct {
	Name      string
	NameBytes []byte
	Alive     bool
	Weight    int
}

func NewMemberNode(name string, alive bool, weight int) *MemberNode {
	return &MemberNode{name, []byte(name), alive, weight}
}

type MemberNodeSet struct {
	nodes []*MemberNode
	//RoundRobin
	*roundRobin
}

func NewMemberNodeSet() *MemberNodeSet {
	return &MemberNodeSet{
		nodes: make([]*MemberNode, 0),
		roundRobin: &roundRobin{},
	}
}

func (m *MemberNodeSet) Add(name string, alive bool, weight int) {
	for i, n := range m.nodes {
		if n.Name == name {
			m.nodes[i] = NewMemberNode(name, alive, weight)
			m.updateRR()
			return
		}
	}
	m.nodes = append(m.nodes, NewMemberNode(name, alive, weight))
	m.updateRR()
}

func (m *MemberNodeSet) Remove(name string) {
	for i, n := range m.nodes {
		if n.Name == name {
			m.nodes = append(m.nodes[:i], m.nodes[i+1:]...)
			m.updateRR()
			return
		}
	}
}

func (m *MemberNodeSet) Length() int {
	return len(m.nodes)
}

func (m *MemberNodeSet) GetAllMemberAddresses(onlyAlive bool) []string {
	rst := make([]string, 0)
	for _, n := range m.nodes {
		if !onlyAlive || (onlyAlive && n.Alive) {
			rst = append(rst, n.Name)
		}
	}
	return rst
}
