package {{basePkgName .TargetPkg}}
{{$modelPkgBase := basePkgName .ModelPkg}}

//
// The contents of this file have been auto-generated. Do not modify.
//

import (
	"{{.ModelPkg}}"
)

{{with nonEmbeddedModels .Models}}
{{range .}}
	{{$modelName := .Name.Public}}

	// {{.Name.Public}}Accessor defines an API for looking up {{.Name.Public}} instances.
	type {{.Name.Public}}Accessor interface {
		{{- range .Fields}}
			{{- if .Flags.IsPK}}
				// Find {{$modelName}} by its primary key.
				Find{{$modelName}}By{{.Name.Public}}({{.Name.Private}} {{.Type.Qualified}}) (*{{$modelPkgBase}}.{{$modelName}}, error)
			{{else if .Flags.IsFindBy}}
				// Find {{$modelName}}s with the specified {{.Name.Private}}.
				Find{{$modelName}}sBy{{.Name.Public}}({{.Name.Private}} {{.Type.Qualified}}) {{$modelName}}Iterator
			{{- end -}}
		{{- end }}
		
		// Count the number of {{$modelName}}s.
		Count{{$modelName}}s() (int, error)

		// Iterate all {{$modelName}}s.
		All{{$modelName}}s() {{$modelName}}Iterator 

		// Create a new {{.Name.Public}} instance and queue it for insertion.
		New{{.Name.Public}}() *{{$modelPkgBase}}.{{.Name.Public}}
	}

	// {{.Name.Public}}Iterator defines an API for iterating lists of {{.Name.Public}} instances.
	type {{.Name.Public}}Iterator interface {
		// Next advances the iterator and returns false if no further data is
		// available or an error occurred. The caller must call Error() to
		// check whether the iteration was aborted due to an error.
		Next() bool

		// {{.Name.Public}} returns the current item. 
		// It should only be invoked if a prior call to Next() returned true.
		// Otherwise, a nil value will be returned.
		{{.Name.Public}}() *{{$modelPkgBase}}.{{.Name.Public}}

		// Error returns the last error (if any) encountered by the iterator.
		Error() error

		// Close the iterator and release any resources associated with it.
		Close() error
	}
{{end}}
{{end}}
