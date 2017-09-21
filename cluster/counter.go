package cluster

type counter struct {
	val int
}

var c = &counter{}

func (c *counter) next() int {
	c.val++
	return int(c.val)
}