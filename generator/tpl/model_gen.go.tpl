package {{basePkgName .TargetPkg}}
{{$modelPkg := .ModelPkg}}

//
// The contents of this file have been auto-generated. Do not modify.
//

{{with requiredPkgImports .Models}}
import (
	{{range .}}
	"{{.}}"
	{{end}}
)
{{end}}

{{with nonEmbeddedModels .Models}}
{{range .}}
	{{$modelName := .Name.Public}}
	type {{$modelName}} struct {
		// An internal field that tracks whether the model is not yet persisted,
		// has been deleted, or if any of its fields have been mutated.
		_flags uint64

		// An internal field that tracks the version of this model.
		_version int64

		{{range .Fields}}
			{{range .Comments}}
				{{- .}}
			{{end -}}
			{{.Name.Private}} {{trimPkgSelector $modelPkg .Type.Local}}
		{{end}}
	}

	// New{{$modelName}} returns a new {{$modelName}} instance with the specified version.
	func New{{$modelName}}(version int64) *{{$modelName}} {
		return &{{$modelName}} {
			_flags: 1 << 0, // flag as not persisted.
			_version: version,
		}
	}
	// Mark the model as deleted.
	func (m *{{$modelName}}) Delete() {
		if !m.IsModelPersisted() {
			return // deleting a non-persisted model is a no-op.
		}
		m._flags |= 1 << 1 // flag as deleted.
	}

	{{range $fieldIndex, $field := .Fields}}
		func (m *{{$modelName}}) {{getter $field}}() {{trimPkgSelector $modelPkg $field.Type.Local}} {
			return m.{{$field.Name.Private}}
		}

		func (m *{{$modelName}}) Set{{$field.Name.Public}}({{$field.Name.Private}} {{trimPkgSelector $modelPkg $field.Type.Local}}) *{{$modelName}} {
			m._flags |= 1 << (2 + {{$fieldIndex}}) // flag i_th field as mutated.
			m.{{.Name.Private}} = {{$field.Name.Private}}
			return m
		}
	{{end}}

	// ModelVersion returns the version of the model as reported by the store.
	func (m *{{$modelName}}) ModelVersion() int64 { return m._version }

	// IsModelPersisted returns true if the model been persisted.
	func (m *{{$modelName}}) IsModelPersisted() bool { return (m._flags & (1 << 0)) == 0 }

	// IsModelDeleted returns true if the model has been marked for deletion.
	func (m *{{$modelName}}) IsModelDeleted() bool { return (m._flags & (1 << 1)) != 0 }

	// IsModelMutated returns true if any of the model fields has been modified.
	func (m *{{$modelName}}) IsModelMutated() bool { return (m._flags & ^uint64(0b11)) != 0 }

	// IsModelFieldMutated returns true if the field at fieldIndex has been modified.
	func (m *{{$modelName}}) IsModelFieldMutated(fieldIndex int) bool { return (m._flags & (1 << (2 + fieldIndex))) != 0 }

	// ResetModelMutationFlags marks the model as persisted, not deleted and non-mutated.
	func (m *{{$modelName}}) ResetModelMutationFlags() { m._flags = 0 }
{{end}}
{{end}}

{{with embeddedModels .Models}}
{{range .}}
	{{$modelName := .Name.Public}}
	type {{$modelName}} struct {
		{{range .Fields}}
			{{range .Comments}}
				{{.}}
			{{- end -}}
			{{/* NOTE: embedded fields define their fields as public */}}
			{{.Name.Public}} {{trimPkgSelector $modelPkg .Type.Local}}
		{{- end -}}
	}
{{end}}
{{end}}
