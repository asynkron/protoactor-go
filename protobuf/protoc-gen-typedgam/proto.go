package main

import (
	"bytes"
	"strings"

	"github.com/gogo/protobuf/protoc-gen-gogo/generator"
)

type ProtoFile struct {
	PackageName string
	Messages    []*ProtoMessage
	Services    []*ProtoService
}

type ProtoMessage struct {
	Name       string
	PascalName string
}

type ProtoService struct {
	Name       string
	PascalName string
	Methods    []*ProtoMethod
}

type ProtoMethod struct {
	Name         string
	PascalName   string
	InputStream  bool
	Input        *ProtoMessage
	OutputStream bool
	Output       *ProtoMessage
}

func ProtoAst(file *generator.FileDescriptor) *ProtoFile {

	pkg := &ProtoFile{}
	pkg.PackageName = file.PackageName()
	messages := make(map[string]*ProtoMessage)
	for _, message := range file.GetMessageType() {
		m := &ProtoMessage{}
		m.Name = message.GetName()
		m.PascalName = MakeFirstLowerCase(m.Name)
		pkg.Messages = append(pkg.Messages, m)
		messages[m.Name] = m
	}

	for _, service := range file.GetService() {
		s := &ProtoService{}
		s.Name = service.GetName()
		s.PascalName = MakeFirstLowerCase(s.Name)
		pkg.Services = append(pkg.Services, s)

		for _, method := range service.GetMethod() {
			m := &ProtoMethod{}
			m.Name = method.GetName()
			m.PascalName = MakeFirstLowerCase(m.Name)
			//		m.InputStream = *method.ClientStreaming
			//		m.OutputStream = *method.ServerStreaming
			input := removePackagePrefix(method.GetInputType(), pkg.PackageName)
			output := removePackagePrefix(method.GetOutputType(), pkg.PackageName)
			m.Input = messages[input]
			m.Output = messages[output]
			s.Methods = append(s.Methods, m)
		}
	}
	return pkg
}

func MakeFirstLowerCase(s string) string {

	if len(s) < 2 {
		return strings.ToLower(s)
	}

	bts := []byte(s)

	lc := bytes.ToLower([]byte{bts[0]})
	rest := bts[1:]

	return string(bytes.Join([][]byte{lc, rest}, nil))
}
