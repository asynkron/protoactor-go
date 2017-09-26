package cluster

type counter struct {
	val int
}

func (c *counter) next() int {
	c.val++
	return int(c.val)
}
