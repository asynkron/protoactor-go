package main

import "github.com/gogo/protobuf/vanity/command"

func main() {

	req := command.Read()
	p := NewGorleans()
	p.Overwrite()

	resp := command.GeneratePlugin(req, p, "_typedgam.go")
	command.Write(resp)
}

// func GeneratePlugin(req *plugin.CodeGeneratorRequest, p generator.Plugin, filenameSuffix string) *plugin.CodeGeneratorResponse {
// 	g := generator.New()
// 	g.Request = req
// 	if len(g.Request.FileToGenerate) == 0 {
// 		g.Fail("no files to generate")
// 	}

// 	g.CommandLineParameters(g.Request.GetParameter())

// 	// g.WrapTypes()
// 	// g.SetPackageNames()
// 	// g.BuildTypeNameMap()
// 	// g.GeneratePlugin(p)

// 	for i := 0; i < len(g.Response.File); i++ {
// 		g.Response.File[i].Name = proto.String(
// 			strings.Replace(*g.Response.File[i].Name, ".pb.go", filenameSuffix, -1),
// 		)
// 	}

// 	return g.Response
// }
