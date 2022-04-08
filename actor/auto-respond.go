package actor

type AutoRespond interface {
	GetAutoResponse(context Context) interface{}
}
