package main

import (
	"bytes"
	"strings"
	"text/template"

	google_protobuf "github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	plugin "github.com/gogo/protobuf/protoc-gen-gogo/plugin"
	"github.com/gogo/protobuf/vanity/command"
)

func main() {
	req := command.Read()
	resp := generateCode(req, "_protoactor.go", true)
	command.Write(resp)
}

func removePackagePrefix(name string, pname string) string {
	return strings.Replace(name, "."+pname+".", "", 1)
}

func generateCode(req *plugin.CodeGeneratorRequest, filenameSuffix string, goFmt bool) *plugin.CodeGeneratorResponse {

	response := &plugin.CodeGeneratorResponse{}
	for _, f := range req.GetProtoFile() {
		s := generate(f)
		fileName := strings.Replace(f.GetName(), ".", "_", 1) + "actor.go"
		r := &plugin.CodeGeneratorResponse_File{
			Content: &s,
			Name:    &fileName,
		}

		response.File = append(response.File, r)
	}

	return response
}

func generate(file *google_protobuf.FileDescriptorProto) string {

	pkg := ProtoAst(file)

	t := template.New("grain")
	t, _ = t.Parse(code)

	var doc bytes.Buffer
	t.Execute(&doc, pkg)
	s := doc.String()

	return s
}
