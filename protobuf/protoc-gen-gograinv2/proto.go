package main

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"

	gogo "github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
)

// code lifted from gogo proto
var isGoKeyword = map[string]bool{
	"break":       true,
	"case":        true,
	"chan":        true,
	"const":       true,
	"continue":    true,
	"default":     true,
	"else":        true,
	"defer":       true,
	"fallthrough": true,
	"for":         true,
	"func":        true,
	"go":          true,
	"goto":        true,
	"if":          true,
	"import":      true,
	"interface":   true,
	"map":         true,
	"package":     true,
	"range":       true,
	"return":      true,
	"select":      true,
	"struct":      true,
	"switch":      true,
	"type":        true,
	"var":         true,
}

// ProtoFile reprpesents a parsed proto file
type ProtoFile struct {
	PackageName string
	Namespace   string
	Messages    []*ProtoMessage
	Services    []*ProtoService
}

// ProtoMessage represents a parsed message in a proto file
type ProtoMessage struct {
	Name       string
	PascalName string
}

// ProtoService represents a parsed service in a proto file
type ProtoService struct {
	Name       string
	PascalName string
	Methods    []*ProtoMethod
}

// ProtoMethod represents a parsed method in a proto service
type ProtoMethod struct {
	Index        int
	Name         string
	PascalName   string
	Input        *ProtoMessage
	Output       *ProtoMessage
	InputStream  bool
	OutputStream bool
}

// ProtoAst transforms a FileDescriptor to an AST that can be used for code generation
func ProtoAst(file *gogo.FileDescriptorProto) *ProtoFile {
	pkg := &ProtoFile{}
	pkg.Namespace = file.GetOptions().GetCsharpNamespace()

	// let us check the option go_package is defined in the file and use that one instead of the
	// default one
	var packageName string
	if file.GetOptions().GetGoPackage() != "" {
		packageName = cleanPackageName(file.GetOptions().GetGoPackage())
	} else {
		packageName = cleanPackageName(file.GetPackage())
	}

	// let us the go package name
	pkg.PackageName = packageName

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

		for i, method := range service.GetMethod() {
			m := &ProtoMethod{}
			m.Index = i
			m.Name = method.GetName()
			m.PascalName = MakeFirstLowerCase(m.Name)
			//		m.InputStream = *method.ClientStreaming
			//		m.OutputStream = *method.ServerStreaming
			input := removePackagePrefix(method.GetInputType(), file.GetPackage())
			output := removePackagePrefix(method.GetOutputType(), file.GetPackage())
			m.Input = messages[input]
			m.Output = messages[output]
			s.Methods = append(s.Methods, m)
		}
	}
	return pkg
}

func goPkgLastElement(full string) string {
	pkgSplitted := strings.Split(full, "/")
	return pkgSplitted[len(pkgSplitted)-1]
}

// MakeFirstLowerCase makes the first character in a string lower case
func MakeFirstLowerCase(s string) string {
	if len(s) < 2 {
		return strings.ToLower(s)
	}

	bts := []byte(s)

	lc := bytes.ToLower([]byte{bts[0]})
	rest := bts[1:]

	return string(bytes.Join([][]byte{lc, rest}, nil))
}

// cleanPackageName lifted from gogo generator
// https://github.com/gogo/protobuf/blob/master/protoc-gen-gogo/generator/generator.go#L695
func cleanPackageName(name string) string {
	parts := strings.Split(name, "/")
	name = parts[len(parts)-1]

	name = strings.Map(badToUnderscore, name)
	// Identifier must not be keyword: insert _.
	if isGoKeyword[name] {
		name = "_" + name
	}
	// Identifier must not begin with digit: insert _.
	if r, _ := utf8.DecodeRuneInString(name); unicode.IsDigit(r) {
		name = "_" + name
	}
	return name
}

// badToUnderscore lifted from gogo generator
func badToUnderscore(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
		return r
	}
	return '_'
}
