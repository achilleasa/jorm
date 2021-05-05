package {{basePkgName .TargetPkg}}

//
// The contents of this file have been auto-generated. Do not modify.
//

import (
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/juju/errors"
	{{with requiredPkgImports .Models}}
	{{range .}}
		"{{.}}"
	{{end}}
	{{end}}

	"{{.ModelPkg}}"
	"{{.StorePkg}}"
)

{{$modelPkg := basePkgName .ModelPkg}}
{{$storePkg := basePkgName .StorePkg}}

var _ {{basePkgName .StorePkg}}.Store = (*InMemory)(nil)

// inMemoryDB stores all model collections for a model UUID. It stores all data
// in memory using a map per model type.
type inMemoryDB struct {
	mu sync.RWMutex
	{{with nonEmbeddedModels .Models}}
	{{- range . -}}
	{{- $modelName := .Name.Public -}}
	{{.Name.Backend}}Collection map[string]*{{$modelPkg}}.{{$modelName}}
	{{end -}}
	{{end -}}
}

func newInMemoryDB() *inMemoryDB {
	return &inMemoryDB{
		{{with nonEmbeddedModels .Models}}
		{{- range . -}}
		{{- $modelName := .Name.Public -}}
		{{.Name.Backend}}Collection: make(map[string]*{{$modelPkg}}.{{$modelName}}),
		{{end -}}
		{{end -}}
	}
}

// InMemory provides a Store interface implementation that keeps its state in memory.
type InMemory struct {
	mu sync.RWMutex
	db map[string]*inMemoryDB
}

// NewInMemory creates a new Inmemory instance.
func NewInMemory() *InMemory {
	return new(InMemory)
}

// Open the store.
func (s *InMemory) Open() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db= make(map[string]*inMemoryDB)
	return nil
}

// Close the store.
func (s *InMemory) Close() error {
	return nil
}

// ModelNamespace returns a ModelNamespace instance scoped to modelUUID.
func (s *InMemory) ModelNamespace(modelUUID string) {{basePkgName .StorePkg}}.ModelNamespace {
	s.mu.Lock()
	defer s.mu.Unlock()

	db := s.db[modelUUID]
	if db == nil {
		db = newInMemoryDB()
		s.db[modelUUID] = db
	}

	return newInMemoryModelNamespace(modelUUID, db)
}

// ApplyChanges applies changes to models tracked by the specified ModelNamespace.
func (s *InMemory) ApplyChanges(ns {{basePkgName .StorePkg}}.ModelNamespace) error {
	inmemNS, ok := ns.(*inMemModelNamespace)
	if !ok {
		return errors.Errorf("unexpected ModelNamespace instance")
	}

	// Check for any leaked iterators
	if len(inmemNS.openIterators) > 0 {
		var callerList []string
		for caller := range inmemNS.openIterators {
			callerList = append(callerList, caller)
		}

		return errors.Errorf("unable to apply changes as the following iterators have not been closed:\n%s", strings.Join(callerList, "\n"))
	}

	// When mutating the store, always acquire the write lock
	s.mu.Lock()
	defer s.mu.Unlock()

	// Since there is no "rollback" support, we need to ensure that all
	// versions match and that there are no PK violations before applying
	// any mutations.
	for ref := range inmemNS.modelRefs {
		mutTracker, ok := ref.({{$storePkg}}.MutationTracker)
		if !ok {
			return errors.Errorf("bug: ModelNamespace holds a reference to a non-model object")
		}
		
		if mutTracker.IsModelPersisted() && (mutTracker.IsModelMutated() || mutTracker.IsModelDeleted()) {
			if err := inmemNS.assertModelVersionMatches(ref); err != nil {
				return err
			}
			continue
		}

		if err := inmemNS.assertModelHasUniquePK(ref); err != nil {
				return err
		}
	}


	// Now apply all mutations.
	for ref := range inmemNS.modelRefs {
		mutTracker := ref.({{$storePkg}}.MutationTracker)
		switch {
		case mutTracker.IsModelDeleted():
			inmemNS.deleteModel(ref)
		case !mutTracker.IsModelPersisted():
			inmemNS.insertModel(ref)
		case mutTracker.IsModelMutated():
			inmemNS.updateModel(ref)
		}
	}

	return nil
}

var _ {{$storePkg}}.ModelNamespace = (*inMemModelNamespace)(nil)

type inMemModelNamespace struct {
	modelUUID string
	backend *inMemoryDB

	// modelRefs keeps track of all model instances that were obtained via
	// this model namespace instance. The backend queries this information
	// to decide what actions it needs to perform when committing changes.
	modelRefs map[interface{}]struct{}

	// The modelCache serves as an index to any modelRefs that were
	// obtained via the backing store. Each entry uses a key of the form
	// "modelType-PK". When attempting to {{$modelPkg}}.unmarshal a model from the store,
	// the implementation will first see if the cache already contains an
	// entry for that PK and return the cached instance instead of creating
	// a new instance. This provides read-your-writes semantics for any
	// operations that use the same ModelNamespace instance.
	modelCache map[string]interface{}

	// The openIterators map tracks iterators that have not been closed yet.
	// It is used to detect leaked iterators and raise errors. The map key
	// is the caller's file:line that created the iterator.
	openIterators map[string]struct{}
}

func newInMemoryModelNamespace(modelUUID string, backend *inMemoryDB) *inMemModelNamespace {
	return &inMemModelNamespace {
		modelUUID: modelUUID,
		backend: backend,
		modelRefs: make(map[interface{}]struct{}),
		modelCache: make(map[string]interface{}),
		openIterators: make(map[string]struct{}),
	}
}

func (ns *inMemModelNamespace) trackModelReference(ref interface{}) {
	ns.modelRefs[ref] = struct{}{}
}

{{$models := .Models}}
{{with nonEmbeddedModels .Models}}
{{range .}}
	{{$model := .}}
	{{$modelName := .Name.Public}}
	{{$modelBackendName := .Name.Backend}}
	//-----------------------------------------
	// {{$modelName}}Accessor implementation
	//-----------------------------------------

	var _ {{$storePkg}}.{{$modelName}}Iterator = (*inMem{{$modelName}}Iterator)(nil)

	type inMem{{$modelName}}Iterator struct {
		iteratorID string
		ns *inMemModelNamespace
		rows []*{{$modelPkg}}.{{$modelName}} // cloned copies off the in-mem DB
		rowIndex int
		cur *{{$modelPkg}}.{{$modelName}}
		closed bool
	}

	func (it *inMem{{$modelName}}Iterator) Error() error { return nil }
	func (it *inMem{{$modelName}}Iterator) {{$modelName}}() *{{$modelPkg}}.{{$modelName}} { return it.cur } 

	func (it *inMem{{$modelName}}Iterator) Close() error { 
		if it.closed {
			return nil
		}
		it.closed = true
		delete(it.ns.openIterators, it.iteratorID)
		return nil
	}

	func (it *inMem{{$modelName}}Iterator) Next() bool {
		if it.rowIndex >= len(it.rows) - 1 {
			return false
		}
		
		it.rowIndex++
		it.cur = it.rows[it.rowIndex]
		return true
	}

	func (ns *inMemModelNamespace) New{{$modelName}}() *{{$modelPkg}}.{{$modelName}} {
		inst := {{$modelPkg}}.New{{$modelName}}(1)
		ns.trackModelReference(inst) // keep track of the object so it can be inserted.
		return inst
	}

	func (ns *inMemModelNamespace) clone{{$modelName}}(inst *{{$modelPkg}}.{{$modelName}}) *{{$modelPkg}}.{{$modelName}} {
		// Check if we have already emitted this model instance from a previous
		// query; if so return the cached value to guarantee read-your-writes
		// semantics for operations sharing the same ModelNamespace instance.
		{{- range $field := $model.Fields -}}
		{{if .Flags.IsPK}}
		cacheKey := fmt.Sprintf("{{$modelName}}_%v", inst.{{getter $field}}())
		if inst, hit := ns.modelCache[cacheKey]; hit {
			return inst.(*{{$modelPkg}}.{{$modelName}})
		}
		{{- end -}}
		{{end}}

		clone := {{$modelPkg}}.New{{$modelName}}(inst.ModelVersion())
		ns.copy{{$modelName}}(inst, clone)

		// Track instance reference so we can apply any mutations to it when
		// we commit any accumulated changes in the model namespace.
		ns.trackModelReference(clone)
		ns.modelCache[cacheKey] = clone
		return clone
	}

	func (ns *inMemModelNamespace) copy{{$modelName}}(src, dst *{{$modelPkg}}.{{$modelName}}) {
		dst {{- range $field := $model.Fields -}}.
			Set{{$field.Name.Public}}(src.{{getter $field}}())
		{{- end}}
		dst.ResetModelMutationFlags()
	}

	{{range $field := $model.Fields}}
		{{if .Flags.IsPK}}
		func (ns *inMemModelNamespace) Find{{$modelName}}By{{$field.Name.Public}}({{$field.Name.Private}} {{$field.Type.Qualified}}) (*{{$modelPkg}}.{{$modelName}}, error) {
			ns.backend.mu.RLock()
			defer ns.backend.mu.RUnlock()

			inst, found := ns.backend.{{$modelBackendName}}Collection[fmt.Sprint({{$field.Name.Private}})]
			if !found {
				return nil, errors.NotFoundf("{{$modelName}} in model %q with {{$field.Name.Public}} %q", ns.modelUUID, {{$field.Name.Private}})
			}

			// Clone and cache item.
			return ns.clone{{$modelName}}(inst), nil
		}
		{{else if .Flags.IsFindBy}}
		func (ns *inMemModelNamespace) Find{{$modelName}}sBy{{$field.Name.Public}}({{$field.Name.Private}} {{$field.Type.Qualified}}) {{$storePkg}}.{{$modelName}}Iterator {
			ns.backend.mu.RLock()
			defer ns.backend.mu.RUnlock()

			var rows []*{{$modelPkg}}.{{$modelName}}
			for _, inst := range ns.backend.{{$modelBackendName}}Collection {
				if inst.{{getter $field}}() != {{$field.Name.Private}} {
					continue
				}
				// Clone and cache item before appending to the result list.
				rows = append(rows, ns.clone{{$modelName}}(inst))
			}
			return ns.make{{$modelName}}Iterator(rows)
		}
		{{end}}
	{{end}}

	func (ns *inMemModelNamespace) Count{{$modelName}}s() (int, error) {
		ns.backend.mu.RLock()
		defer ns.backend.mu.RUnlock()
		return len(ns.backend.{{$modelBackendName}}Collection), nil
	}

	func (ns *inMemModelNamespace) All{{$modelName}}s() {{$storePkg}}.{{$modelName}}Iterator {
		var rows []*{{$modelPkg}}.{{$modelName}}
		for _, inst := range ns.backend.{{$modelBackendName}}Collection {
			// Clone and cache item before appending to the result list.
			rows = append(rows, ns.clone{{$modelName}}(inst))
		}
		return ns.make{{$modelName}}Iterator(rows)
	}

	func (ns *inMemModelNamespace) make{{$modelName}}Iterator(rows []*{{$modelPkg}}.{{$modelName}}) {{$storePkg}}.{{$modelName}}Iterator {
		_, file, line, _ := runtime.Caller(2)
		iteratorID := fmt.Sprintf("%s:%d", file, line)

		res := &inMem{{$modelName}}Iterator{
			iteratorID: iteratorID,
			ns: ns,
			rows: rows,
			rowIndex: -1,
		}
		
		ns.openIterators[iteratorID] = struct{}{}
		return res
	}
{{end}}
{{end}}

// assertModelVersionMatches ensures that the model's version in the store matches
// the version in the specified instance.
//
// This method must be called while holding a read lock in the backing store.
func (ns *inMemModelNamespace) assertModelVersionMatches(modelInst interface{}) error {
	var (
		instVersion int64
		storeVersion int64
	)

	switch t := (modelInst).(type){
		{{with nonEmbeddedModels .Models}}
		{{- range .}}
		{{- $modelName := .Name.Public -}}
		{{- $modelBackendName := .Name.Backend -}}
		case *{{$modelPkg}}.{{$modelName}}:
			instVersion = t.ModelVersion()
			{{- range .Fields -}}{{if .Flags.IsPK}}
			if stInst, found := ns.backend.{{$modelBackendName}}Collection[fmt.Sprint(t.Get{{.Name.Public}}())]; found {
				storeVersion = stInst.ModelVersion()
			}
			{{end}}{{end -}}
		{{end -}}
		{{end -}}
		default:
			return errors.Errorf("unsupported instance type %T", t)
	}

	if instVersion != storeVersion {
		return {{$storePkg}}.ErrStoreVersionMismatch
	}
	return nil
}

func (ns *inMemModelNamespace) assertModelHasUniquePK(modelInst interface{}) error {
	var pkExists bool
	switch t := (modelInst).(type){
		{{with nonEmbeddedModels .Models}}
		{{- range .}}
		{{- $modelName := .Name.Public -}}
		{{- $modelBackendName := .Name.Backend -}}
		case *{{$modelPkg}}.{{$modelName}}:
			{{- range .Fields -}}{{if .Flags.IsPK}}
				if _, pkExists = ns.backend.{{$modelBackendName}}Collection[fmt.Sprint(t.Get{{.Name.Public}}())]; pkExists {
					return errors.Errorf("PK key violation for {{$modelName}}:  PK %v already exists", t.Get{{.Name.Public}}())
				}
			{{end -}}
			{{end -}}
		{{end -}}
		{{end -}}
	}
	// PK is unique
	return nil
}

func (ns *inMemModelNamespace) insertModel(modelInst interface{}) {
	// insertModel assumes the caller has already checked for PK violations.
	switch t := (modelInst).(type){
		{{with nonEmbeddedModels .Models}}
		{{- range .}}
		{{- $modelName := .Name.Public -}}
		{{- $modelBackendName := .Name.Backend -}}
		case *{{$modelPkg}}.{{$modelName}}:
			{{- range .Fields -}}{{if .Flags.IsPK}}
				clone := {{$modelPkg}}.New{{$modelName}}(t.ModelVersion())
				ns.copy{{$modelName}}(t, clone)
				ns.backend.{{$modelBackendName}}Collection[fmt.Sprint(clone.Get{{.Name.Public}}())] = clone
			{{end -}}
			{{end -}}
		{{end -}}
		{{end -}}
	}
}

func (ns *inMemModelNamespace) updateModel(modelInst interface{}) {
	switch t := (modelInst).(type){
		{{with nonEmbeddedModels .Models}}
		{{- range .}}
		{{- $modelName := .Name.Public -}}
		{{- $modelBackendName := .Name.Backend -}}
		case *{{$modelPkg}}.{{$modelName}}:
			{{- range .Fields -}}{{if .Flags.IsPK}}
				clone := {{$modelPkg}}.New{{$modelName}}(t.ModelVersion()+1) // bump version
				ns.copy{{$modelName}}(t, clone)
				ns.backend.{{$modelBackendName}}Collection[fmt.Sprint(clone.Get{{.Name.Public}}())] = clone
			{{end -}}
			{{end -}}
		{{end -}}
		{{end -}}
	}
}

func (ns *inMemModelNamespace) deleteModel(modelInst interface{}) {
	switch t := (modelInst).(type){
		{{with nonEmbeddedModels .Models}}
		{{- range .}}
		{{- $modelName := .Name.Public -}}
		{{- $modelBackendName := .Name.Backend -}}
		case *{{$modelPkg}}.{{$modelName}}:
			{{- range .Fields -}}{{if .Flags.IsPK}}
				delete(ns.backend.{{$modelBackendName}}Collection, fmt.Sprint(t.Get{{.Name.Public}}()))
			{{end -}}
			{{end -}}
		{{end -}}
		{{end -}}
	}
}
	
// ModelUUID() returns the Juju model UUID associated with this instance.
func (ns  *inMemModelNamespace) ModelUUID() string {
	return ns.modelUUID		
}
