package index

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	gotoken "go/token"
	"path/filepath"
	"strings"
)

func analyzeGo(file *FileInfo, content []byte) {
	file.IsTest = strings.HasSuffix(file.Path, "_test.go")
	file.IsRoute = routeLikePath(file.Path)

	fset := gotoken.NewFileSet()
	astFile, err := parser.ParseFile(fset, file.AbsPath, content, parser.ParseComments)
	if err != nil {
		return
	}
	file.Package = astFile.Name.Name

	for _, imported := range astFile.Imports {
		if imported.Path != nil {
			file.Imports = append(file.Imports, strings.Trim(imported.Path.Value, `"`))
		}
	}

	for _, decl := range astFile.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			kind := "func"
			if d.Recv != nil {
				kind = "method"
			}
			file.Symbols = append(file.Symbols, Symbol{
				Name:      d.Name.Name,
				Kind:      kind,
				Signature: formatFuncSignature(d),
				Line:      fset.Position(d.Pos()).Line,
				Exported:  ast.IsExported(d.Name.Name),
			})
		case *ast.GenDecl:
			if d.Tok != gotoken.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				file.Symbols = append(file.Symbols, Symbol{
					Name:      typeSpec.Name.Name,
					Kind:      goTypeKind(typeSpec.Type),
					Signature: formatTypeSignature(typeSpec),
					Line:      fset.Position(typeSpec.Pos()).Line,
					Exported:  ast.IsExported(typeSpec.Name.Name),
				})
			}
		}
	}

	if !file.IsRoute {
		base := strings.ToLower(filepath.Base(file.Path))
		file.IsRoute = strings.Contains(file.Content, "http.") ||
			strings.Contains(file.Content, "router.") ||
			strings.Contains(file.Content, ".HandleFunc(") ||
			strings.Contains(base, "handler")
	}
}

func goTypeKind(expr ast.Expr) string {
	switch expr.(type) {
	case *ast.StructType:
		return "struct"
	case *ast.InterfaceType:
		return "interface"
	default:
		return "type"
	}
}

func formatFuncSignature(decl *ast.FuncDecl) string {
	var b strings.Builder
	b.WriteString("func ")
	if decl.Recv != nil {
		b.WriteString("(")
		b.WriteString(formatFields(decl.Recv.List))
		b.WriteString(") ")
	}
	b.WriteString(decl.Name.Name)
	b.WriteString("(")
	if decl.Type.Params != nil {
		b.WriteString(formatFields(decl.Type.Params.List))
	}
	b.WriteString(")")
	if decl.Type.Results != nil && len(decl.Type.Results.List) > 0 {
		b.WriteString(" ")
		if len(decl.Type.Results.List) == 1 && len(decl.Type.Results.List[0].Names) == 0 {
			b.WriteString(renderNode(decl.Type.Results.List[0].Type))
		} else {
			b.WriteString("(")
			b.WriteString(formatFields(decl.Type.Results.List))
			b.WriteString(")")
		}
	}
	return compactLine(b.String())
}

func formatFields(fields []*ast.Field) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		typeText := renderNode(field.Type)
		if len(field.Names) == 0 {
			parts = append(parts, typeText)
			continue
		}
		names := make([]string, 0, len(field.Names))
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
		parts = append(parts, strings.Join(names, ", ")+" "+typeText)
	}
	return strings.Join(parts, ", ")
}

func formatTypeSignature(spec *ast.TypeSpec) string {
	switch t := spec.Type.(type) {
	case *ast.StructType:
		return "type " + spec.Name.Name + " struct { " + fieldSummary(t.Fields) + " }"
	case *ast.InterfaceType:
		return "type " + spec.Name.Name + " interface { " + fieldSummary(t.Methods) + " }"
	default:
		return compactLine("type " + spec.Name.Name + " " + renderNode(spec.Type))
	}
}

func fieldSummary(list *ast.FieldList) string {
	if list == nil || len(list.List) == 0 {
		return ""
	}
	parts := make([]string, 0, len(list.List))
	for i, field := range list.List {
		if i >= 8 {
			parts = append(parts, "...")
			break
		}
		typeText := renderNode(field.Type)
		if len(field.Names) == 0 {
			parts = append(parts, typeText)
			continue
		}
		names := make([]string, 0, len(field.Names))
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
		parts = append(parts, strings.Join(names, ", ")+" "+typeText)
	}
	return strings.Join(parts, "; ")
}

func renderNode(node ast.Node) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, gotoken.NewFileSet(), node)
	return buf.String()
}

func compactLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
