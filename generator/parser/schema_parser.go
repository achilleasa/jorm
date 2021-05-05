package parser

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/fatih/structtag"
	"golang.org/x/tools/go/loader"
)

// Model describes a model that corresponds to a public struct in the schema
// package.
type Model struct {
	Name struct {
		// The model name. The value is set to the name of the struct
		// that containts the model definition.
		Public string

		// The table/collection name used by the backend to
		// store instances of this model.
		Backend string
	}

	// The source file where the model definition was parsed from.
	SrcFile string

	// True if this a model is meant to be embedded within another model.
	// Every model that does not define a primary key via a schema tag is
	// treated as an embedded model and will not be assigned a dedicated
	// table/collection in the backing store.
	IsEmbdedded bool

	// The list of fields associated with the model.
	Fields []*Field
}

// HasFindByFields returns true if any of the model fields has a "find_by" flag.
func (m *Model) HasFindByFields() bool {
	for _, field := range m.Fields {
		if field.Flags.IsFindBy {
			return true
		}
	}

	return false
}

// HasNullableFields returns true if any of the model fields can be set to nil.
func (m *Model) HasNullableFields() bool {
	for _, field := range m.Fields {
		if field.Flags.IsNullable {
			return true
		}
	}

	return false
}

// Field describes an individual model field.
type Field struct {
	// The Go comment lines associated with the field definition in the
	// schema.
	Comments []string

	Name struct {
		// Set to the field name from the struct that the model was
		// parsed from.
		Public string

		// A (lower) camel-cased variant of the public field name that
		// is suitable for using as variable or argument names.
		Private string

		// Specifies the name of the field to be used by the backend
		// for referring to the field. If not specified via a schema
		// tag, the Private field name will be used by default.
		Backend string
	}

	Type struct {
		// The package-local type for the field. This is meant to be
		// used instead of Qualified when refer to field types within
		// the model package so that we don't cause import cycles if
		// the type references another type defined in the model package.
		Local string

		// The fully-qualified Go type of the field including package
		// selectors. It can be used to refer to the field type from
		// outside the model package.
		Qualified string

		// Similar to Type.Qualified but with any type aliases resolved
		// to their underlying type. For example, given the following
		// types:
		//
		//	type Flag uint8
		//	type Entry struct {
		//		Options Flag
		// 	}
		//
		// The Qualified type for the "Options" field will be set to
		// "Flag" whereas the Resolved type will be set to "uint8".
		Resolved string

		// If the type includes any external package type references,
		// RequiredImports will hold the list of package imports that
		// are required to reference this type from other packages.
		RequiredImports []string
	}

	Flags struct {
		// True if the field is a primary key for the model.
		IsPK bool

		// True if instances of this model can be looked up via this
		// field.
		IsFindBy bool

		/// True if a nil value can be specified for the field
		IsNullable bool
	}
}

// Models holds the list of model schemas that have been parsed from a
// particular Go package.
type Models struct {
	allModels []*Model
}

// ParseModelSchemas analyzes the files in the schema package, extracts any
// model definitions and returns them back as a *Models instance.
func ParseModelSchemas(schemaPkgName, modelPkgName string) (*Models, error) {
	fset, pkgInfo, err := loadPkg(schemaPkgName)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve type information from package %q: %w", schemaPkgName, err)
	}

	structASTs := extractStructASTs(fset, pkgInfo.Files)
	models, err := parseModelSchemas(fset, structASTs, pkgInfo.Uses, modelPkgName)
	if err != nil {
		return nil, fmt.Errorf("unable to parse model schemas from package %q: %w", schemaPkgName, err)
	}
	return models, nil
}

// AllModels returns a list containing all models.
func (m *Models) All() []*Model {
	return m.allModels
}

// ModelsBySrcFile groups Model definitions by the name of the file they were
// defined in and returns them as a map where keys are paths to the schema
// definition source files.
func (m *Models) ModelsBySrcFile() map[string][]*Model {
	modelsBySrc := make(map[string][]*Model)
	for _, model := range m.allModels {
		modelsBySrc[model.SrcFile] = append(modelsBySrc[model.SrcFile], model)
	}

	return modelsBySrc
}

// loadPkg uses go/loader to compile pkgName (including any of its
// direct and indirect dependencies) and returns back the obtained ASTs and type
// information.
func loadPkg(pkgName string) (*token.FileSet, *loader.PackageInfo, error) {
	var loaderConf loader.Config
	_, err := loaderConf.FromArgs([]string{pkgName}, false)
	if err != nil {
		return nil, nil, err
	}
	prog, err := loaderConf.Load()
	if err != nil {
		return nil, nil, err
	}

	return prog.Fset, prog.Package(pkgName), nil
}

// structAST represents a parsed model struct definition.
type structAST struct {
	// The name of the struct identifier.
	Name string

	// The file where the struct was defined in
	SrcFile string

	// The AST for the struct.
	Decl *ast.StructType
}

// extractStructASTs returns a list of struct ASTs within a package that
// define models.
func extractStructASTs(fset *token.FileSet, fileASTs []*ast.File) []structAST {
	var structASTs []structAST
	for _, fileAST := range fileASTs {
		for _, decl := range fileAST.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}

				structASTs = append(structASTs,
					structAST{
						Name:    typeSpec.Name.Name,
						SrcFile: fset.Position(fileAST.Pos()).Filename,
						Decl:    structType,
					},
				)
			}
		}
	}

	return structASTs
}

// parseModelSchemas processes the provided struct ASTs and maps them into
// a list of Model instances.
func parseModelSchemas(fset *token.FileSet, structASTs []structAST, typeInfo map[*ast.Ident]types.Object, modelPkgName string) (*Models, error) {
	var models Models

	for _, structAST := range structASTs {
		structName := structAST.Name

		// Ignore un-exported types and empty structs
		if strings.Title(structName) != structName || structAST.Decl.Fields == nil {
			continue
		}

		model := &Model{
			SrcFile:     structAST.SrcFile,
			IsEmbdedded: true, // models are embedded by default unless we find a PK schema tag
		}
		model.Name.Public = structName
		model.Name.Backend = fmt.Sprintf("%ss", lcIdentifier(structName))

		for _, fieldAST := range structAST.Decl.Fields.List {
			if len(fieldAST.Names) != 1 {
				continue
			}
			fieldName := fieldAST.Names[0].Name

			// Resolve field type
			localType, fqType, resolvedType, requiredImports, err := resolveType(fset, fieldAST.Type, typeInfo, modelPkgName)
			if err != nil {
				return nil, fmt.Errorf("unable to resolve type for field %q in struct %q: %w", fieldName, structName, err)
			}

			field := new(Field)
			field.Name.Public = fieldName
			field.Name.Private = lcIdentifier(fieldName)
			field.Name.Backend = lcIdentifier(fieldName)
			field.Type.Local = localType
			field.Type.Qualified = fqType
			field.Type.Resolved = resolvedType
			field.Type.RequiredImports = requiredImports

			// Pointer fields, maps and slices are considered nullable.
			field.Flags.IsNullable = strings.HasPrefix(fqType, "*") ||
				strings.HasPrefix(fqType, "map") ||
				strings.HasPrefix(fqType, "[]")

			// Track comments
			if fieldAST.Doc != nil {
				for _, comment := range fieldAST.Doc.List {
					field.Comments = append(field.Comments, comment.Text)
				}
			}

			// Process schema tags
			var tagVal string
			if fieldAST.Tag != nil {
				tagVal = fieldAST.Tag.Value
			}

			fieldTag, err := structtag.Parse(strings.Replace(tagVal, "`", "", -1))
			if err != nil {
				return nil, fmt.Errorf("unable to parse tag for field %q in struct %q: %w", fieldName, structName, err)
			}

			if schemaTag, err := fieldTag.Get("schema"); err == nil {
				field.Name.Backend = schemaTag.Name
				for _, opt := range schemaTag.Options {
					switch opt {
					case "pk":
						field.Flags.IsPK = true
						model.IsEmbdedded = false
					case "find_by":
						field.Flags.IsFindBy = true
					default:
						return nil, fmt.Errorf("unsupported schema option %q for field %q in struct %q", opt, fieldName, structName)
					}
				}
			}

			model.Fields = append(model.Fields, field)
		}

		models.allModels = append(models.allModels, model)
	}

	return &models, nil
}

// lcIdentifier returns a suitably lowercased version of v that can serve as an
// unexported field name in a Go struct. The implementation is smart enough to
// properly lowercase identifiers acronym-like idents such as "ID" and "UUID".
func lcIdentifier(v string) string {
	if len(v) == 0 {
		return v
	}

	// Find the longest prefix len containing uppercase chars and lowercase
	// it. This ensures that identifiers such as ID or UUID convert correctly.
	var ucLen int
	var r rune
	for ucLen, r = range v {
		if !unicode.IsUpper(r) {
			break
		}
	}

	return strings.ToLower(v[:ucLen+1]) + v[ucLen+1:]
}

// resolveType scans fieldType and returns back:
// - the pkg-local go type
// - the
// fully qualified go type, the fully qualified type with type aliases replaced
// and the list of imports for all packages used by the type components.
func resolveType(fset *token.FileSet, fieldType ast.Node, typeInfo map[*ast.Ident]types.Object, modelPkgName string) (string, string, string, []string, error) {
	baseModelPkgName := filepath.Base(modelPkgName)

	// Use the go/printer to construct the fully qualified Go type for the
	// field and prefix references to local types within the schema package
	// with the model package where the model types will live.
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, fieldType); err != nil {
		return "", "", "", nil, err
	}
	localType := buf.String()
	fqType := addPkgPrefixToLocalTypes(baseModelPkgName, localType)

	// Visit the type AST and collect the required imports
	// for using this type in another package.
	resolver := &typeResolver{
		fset:     fset,
		typeInfo: typeInfo,
	}
	ast.Walk(resolver, fieldType)

	var imports []string
	for pkg := range resolver.imports {
		imports = append(imports, pkg)
	}

	return localType, fqType, resolver.resolvedType.String(), imports, nil
}

// addPkgPrefixToLocalTypes scans the provided goType for types that are
// defined within the schema package and injects a "$pkg." prefix so the
// types can correctly function when referring to the models that will be
// created in the provided pkg.
func addPkgPrefixToLocalTypes(pkg string, goType string) string {
	var (
		patchedType bytes.Buffer
		lastCh      rune
	)
	for i, nextCh := range goType {
		// Look for an uppercase character that is either the start
		// of the string or is preceded by a bracket character.
		prevIsBracket := i > 0 && (lastCh == '[' || lastCh == ']')
		if unicode.IsUpper(nextCh) && (i == 0 || prevIsBracket) {
			patchedType.WriteString(pkg)
			patchedType.WriteRune('.')
		}
		patchedType.WriteRune(nextCh)

		lastCh = nextCh
	}

	return patchedType.String()
}

// typeResolver implements ast.Visitor. It recursively processes a type
// definition and:
// 1) replaces type aliases with their underlying types.
// 2) replace references to types within $schemaPkgName with $modelPkgName
//    so that the types can be safely relocated to the model package.
// 3) records the package imports for any non-built-in referenced type part.
type typeResolver struct {
	fset     *token.FileSet
	typeInfo map[*ast.Ident]types.Object

	imports        map[string]struct{}
	resolvedType   bytes.Buffer
	visitingMapKey bool
}

func (c *typeResolver) Visit(n ast.Node) ast.Visitor {
	switch t := n.(type) {
	case *ast.StarExpr:
		c.resolvedType.WriteRune('*')
		return c // Continue recursing without terminating open map literals
	case *ast.MapType:
		c.resolvedType.WriteString("map[")
		defer func() {
			c.visitingMapKey = true
		}()
	case *ast.ArrayType:
		c.resolvedType.WriteString("[]")
	case *ast.SelectorExpr:
		if identExpr, isIdentExpr := t.X.(*ast.Ident); isIdentExpr {
			// Record import for the package defining the type
			if pkgName, isPkgName := c.typeInfo[identExpr].(*types.PkgName); isPkgName {
				if c.imports == nil {
					c.imports = make(map[string]struct{})
				}
				c.imports[pkgName.Imported().Path()] = struct{}{}
			}
		}

		if typeInfo, foundTypeInfo := c.typeInfo[t.Sel].(*types.TypeName); foundTypeInfo {
			// If the imported type is an alias to a basic Go type,
			// record that; otherwise just record the actual type.
			if basicType, isBasicType := typeInfo.Type().Underlying().(*types.Basic); isBasicType {
				c.resolvedType.WriteString(basicType.String())
			} else {
				printer.Fprint(&c.resolvedType, c.fset, t.Sel)
			}
			// We have already recorded the underlying type; no need
			// to recurse deeper.
			return nil
		}
	case *ast.Ident:
		if typeInfo, foundTypeInfo := c.typeInfo[t].(*types.TypeName); foundTypeInfo {
			// If the imported type is an alias to a basic Go type,
			// record that; otherwise just record the actual type.
			if basicType, isBasicType := typeInfo.Type().Underlying().(*types.Basic); isBasicType {
				c.resolvedType.WriteString(basicType.String())
			} else {
				printer.Fprint(&c.resolvedType, c.fset, n)
			}
		} else {
			c.resolvedType.WriteString(t.Name)
		}
	}

	if c.visitingMapKey {
		c.resolvedType.WriteRune(']')
		c.visitingMapKey = false
	}

	return c
}
