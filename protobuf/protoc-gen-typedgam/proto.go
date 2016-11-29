package main

type ProtoFile struct {
	PackageName string
	Messages    []*ProtoMessage
	Services    []*ProtoService
}

type ProtoMessage struct {
	Name string
}

type ProtoService struct {
	Name    string
	Methods []*ProtoMethod
}

type ProtoMethod struct {
	Name         string
	InputStream  bool
	Input        *ProtoMessage
	OutputStream bool
	Output       *ProtoMessage
}
