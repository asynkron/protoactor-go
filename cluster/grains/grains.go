package grains

type GrainMixin struct {
	id string
}

func (gm GrainMixin) Id() string {
	return gm.id
}
