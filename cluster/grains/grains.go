package grains

type GrainMixin struct {
	id string
}

func NewGrainMixin(id string) GrainMixin {
	return GrainMixin{
		id: id,
	}
}

func (gm GrainMixin) Id() string {
	return gm.id
}
