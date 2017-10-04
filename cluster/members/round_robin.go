package members

type roundRobin struct {
	currIndex  int
	currWeight int
	maxWeight  int
	gcd        int
}

func (m *MemberNodeSet) GetByRoundRobin() string {

	l := len(m.nodes)

	if l == 0 {
		return ""
	}

	if l == 1 {
		return m.nodes[0].Name
	}

	for {
		m.currIndex = (m.currIndex + 1) % l
		if m.currIndex == 0 {
			m.currWeight = m.currWeight - 1
			if m.currWeight <= 0 {
				m.currWeight = m.maxWeight
			}
		}
		if m.nodes[m.currIndex].Weight >= m.currWeight {
			return m.nodes[m.currIndex].Name
		}
	}
}

func (m *MemberNodeSet) updateRR() {
	m.maxWeight = m.getMaxWeight()
	//m.gcd = m.getGcd()
}

func (m *MemberNodeSet) getMaxWeight() int {
	max := 0
	for _, n := range m.nodes {
		if n.Weight > max {
			max = n.Weight
		}
	}
	return max
}

/*
func (m *MemberNodeSet) getGcd() int {
	if len(m.nodes) == 0 {
		return 0
	}
	ints := make([]int, len(m.nodes))
	for i, n := range m.nodes {
		ints[i] = n.Weight
	}
	return ngcd(ints)
}

func gcd(a, b int) int {
	if a < b {
		a, b = b, a
	}
	if b == 0 {
		return a
	}
	return gcd(b, a%b)
}

func ngcd(ints []int) int {
	n := len(ints)
	if n == 1 {
		return ints[0]
	}
	return gcd(ints[n-1], ngcd(ints[0:n-1]))
}
*/
