package {{basePkgName .TargetPkg}}

//
// The contents of this file have been auto-generated. Do not modify.
//

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/juju/errors"
	"github.com/juju/mgo/v2"
	"github.com/juju/mgo/v2/bson"
	"github.com/juju/mgo/v2/sstxn"
	"github.com/juju/mgo/v2/txn"
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

var _ {{$storePkg}}.Store = (*Mongo)(nil)

type txnRunner interface {
	Run(ops []txn.Op, id bson.ObjectId, info interface{}) (err error)
}

// Mongo provides a Store interface implementation that is backed by mongodb.
type Mongo struct {
	mongoURI string
	dbName string
	useSSTXN bool

	mu sync.RWMutex
	mgoSession *mgo.Session
	mgoDB *mgo.Database
}

// NewMongo creates a new Mongo instance backed by dbFile.
func NewMongo(mongoURI, dbName string, useSSTXN bool) *Mongo {
	return &Mongo{
		mongoURI: mongoURI,
		dbName: dbName,
		useSSTXN: useSSTXN,
	}
}

// Open the store.
func (s *Mongo) Open() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	if s.mgoSession, err = mgo.DialWithTimeout(s.mongoURI, 5 * time.Second); err != nil {
		return err
	}

	s.mgoDB = s.mgoSession.DB(s.dbName)
	return s.ensureIndices()
}

// ensureIndices creates the indices for all find_by fields.
func (s *Mongo) ensureIndices() error {
{{- with nonEmbeddedModels .Models}}
	var col *mgo.Collection

{{range . -}}
	{{$model := .}}
	{{- $modelName := .Name.Public -}}
	{{- $modelBackendName := .Name.Backend -}}
		// Table indices for the {{$modelName}} collection.
		col = s.mgoDB.C("{{$modelBackendName}}")
		if err := col.EnsureIndexKey("model-uuid"); err != nil {
			return errors.Annotate(err, `creating mongo index for field "model-uuid" of collection "{{$modelBackendName}}"`)
		}
		{{range $field := $model.Fields -}}{{- if $field.Flags.IsFindBy -}}
		if err := col.EnsureIndexKey("model-uuid", "{{$field.Name.Backend}}"); err != nil {
			return errors.Annotate(err, `creating mongo index for field "{{$field.Name.Backend}}" of collection "{{$modelBackendName}}"`)
		}
		{{end -}}
		{{end -}}
{{end -}}
{{end}}

	return nil
}

// Close the store.
func (s *Mongo) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.mgoSession != nil {
		s.mgoSession.Close()
		s.mgoSession = nil
	}
	return nil
}

// ModelNamespace returns a ModelNamespace instance scoped to modelUUID.
func (s *Mongo) ModelNamespace(modelUUID string) {{$storePkg}}.ModelNamespace {
	return newMongoModelNamespace(modelUUID, s)
}

// Mongo provides a Store interface implementation that is backed by mongodb.
func (s *Mongo) ApplyChanges(ns {{$storePkg}}.ModelNamespace) error {
	mongoNS, ok := ns.(*mongoModelNamespace)
	if !ok {
		return errors.Errorf("unexpected ModelNamespace instance")
	}

	// Check for any leaked iterators
	if len(mongoNS.openIterators) > 0 {
		var callerList []string
		for caller := range mongoNS.openIterators {
			callerList = append(callerList, caller)
		}

		return errors.Errorf("unable to apply changes as the following iterators have not been closed:\n%s", strings.Join(callerList, "\n"))
	}

	// Assemble ops including any required assertions.
	var ops []txn.Op
	for ref := range mongoNS.modelRefs {
		mutTracker, ok := ref.({{$storePkg}}.MutationTracker)
		if !ok {
			return errors.Errorf("bug: ModelNamespace holds a reference to a non-model object")
		}

		var (
			op txn.Op
			err error
		)

		switch {
		case mutTracker.IsModelDeleted():
			op, err = mongoNS.deleteModelOp(ref)
		case !mutTracker.IsModelPersisted():
			op, err = mongoNS.insertModelOp(ref)
		case mutTracker.IsModelMutated():
			op, err = mongoNS.updateModelOp(ref)
		}

		if err != nil {
			return errors.Trace(err)
		}
		ops = append(ops, op)
	}

	if err := s.getTxnRunner().Run(ops, "", nil); err != nil {
		if err == txn.ErrAborted {
			return {{$storePkg}}.ErrStoreVersionMismatch
		}
		return err
	}
	return nil
}

func (s *Mongo) getTxnRunner() txnRunner {
	s.mu.RLock() 
	defer s.mu.RUnlock()

	if s.useSSTXN {
		return sstxn.NewRunner(s.mgoDB, nil)
	}

	txnCol := s.mgoDB.C("txns")
	return txn.NewRunner(txnCol)
}

var _ {{$storePkg}}.ModelNamespace = (*mongoModelNamespace)(nil)

type mongoModelNamespace struct {
	modelUUID string
	backend *Mongo

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

func newMongoModelNamespace(modelUUID string, backend *Mongo) *mongoModelNamespace {
	return &mongoModelNamespace {
		modelUUID: modelUUID,
		backend: backend,
		modelRefs: make(map[interface{}]struct{}),
		modelCache: make(map[string]interface{}),
		openIterators: make(map[string]struct{}),
	}
}

func (ns *mongoModelNamespace) trackModelReference(ref interface{}) {
	ns.modelRefs[ref] = struct{}{}
}

{{$models := .Models}}
{{with nonEmbeddedModels .Models}}
{{range .}}
	{{- $model := . -}}
	{{- $modelName := .Name.Public -}}
	{{- $modelBackendName := .Name.Backend -}}
	//-----------------------------------------
	// {{$modelName}}Accessor implementation
	//-----------------------------------------

	var _ {{$storePkg}}.{{$modelName}}Iterator = (*mongo{{$modelName}}Iterator)(nil)

	type mongo{{$modelName}}Iterator struct {
		iteratorID string
		ns *mongoModelNamespace
		mgoIt *mgo.Iter
		cur *{{$modelPkg}}.{{$modelName}}
		lastErr error
		closed bool
	}

	func (it *mongo{{$modelName}}Iterator) Error() error { return it.lastErr }
	func (it *mongo{{$modelName}}Iterator) {{$modelName}}() *{{$modelPkg}}.{{$modelName}} { return it.cur }

	func (it *mongo{{$modelName}}Iterator) Close() error { 
		if it.closed {
			return it.lastErr
		}
		it.closed = true
		it.lastErr = it.mgoIt.Close() 
		delete(it.ns.openIterators, it.iteratorID)
		return it.lastErr
	}

	func (it *mongo{{$modelName}}Iterator) Next() bool {
		if it.lastErr != nil {
			return false
		} else if itErr := it.mgoIt.Err(); itErr != nil {
			it.lastErr = itErr
			return false
		} else if it.mgoIt.Done() {
			return false
		}

		// Unfortunately, the mgo.Iterator.Next is not compatible with
		// the Query.One signature so we need to adapt it.
		if it.cur, it.lastErr = it.ns.unmarshal{{$modelName}}(func(dst interface{}) error {
			if !it.mgoIt.Next(dst) {
				return errors.Errorf("{{$modelName}} iterator returned no data even though more data seems to be available")
			}
			return nil
		});it.lastErr != nil {
			return false
		}
		return true
	}

	func (ns *mongoModelNamespace) New{{$modelName}}() *{{$modelPkg}}.{{$modelName}} {
		inst := {{$modelPkg}}.New{{$modelName}}(2) // mgo/txn uses 2 as the initial version.
		ns.trackModelReference(inst) // keep track of the object so it can be inserted.
		return inst
	}

	{{range $field := $model.Fields}}
		{{if .Flags.IsPK}}
		func (ns *mongoModelNamespace) Find{{$modelName}}By{{$field.Name.Public}}({{$field.Name.Private}} {{$field.Type.Qualified}}) (*{{$modelPkg}}.{{$modelName}}, error) {
			ns.backend.mu.RLock()
			defer ns.backend.mu.RUnlock()

			// Check if we have already emitted this model instance from a previous
			// query; if so return the cached value to avoid making a query and to
			// provide read-your-writes semantics for operations sharing the same
			// ModelNamespace instance.
			cacheKey := fmt.Sprintf("{{$modelName}}_%v", {{$field.Name.Private}})
			if inst, hit := ns.modelCache[cacheKey]; hit {
				return inst.(*{{$modelPkg}}.{{$modelName}}), nil
			}

			query := ns.backend.mgoDB.C("{{$modelBackendName}}").Find(bson.M{"_id": makeMongoID(ns.modelUUID, {{$field.Name.Private}})})
			inst, err := ns.unmarshal{{$modelName}}(query.One)
			if err == mgo.ErrNotFound {
				return nil, errors.NotFoundf("{{$modelName}} in model %q with {{$field.Name.Public}} %q", ns.modelUUID, {{$field.Name.Private}})
			}
			return inst, err
		}
		{{else if .Flags.IsFindBy}}
		func (ns *mongoModelNamespace) Find{{$modelName}}sBy{{$field.Name.Public}}({{$field.Name.Private}} {{$field.Type.Qualified}}) {{$storePkg}}.{{$modelName}}Iterator {
			ns.backend.mu.RLock()
			defer ns.backend.mu.RUnlock()

			query := ns.backend.mgoDB.C("{{$modelBackendName}}").Find(bson.M{
				"{{$field.Name.Backend}}": {{$field.Name.Private}},
				"model-uuid": ns.modelUUID,
			})
			return ns.make{{$modelName}}Iterator(query)
		}
		{{end}}
	{{end}}

	func (ns *mongoModelNamespace) Count{{$modelName}}s() (int, error) {
		ns.backend.mu.RLock()
		defer ns.backend.mu.RUnlock()

		return ns.backend.mgoDB.C("{{$modelBackendName}}").Find(bson.M{
			"model-uuid": ns.modelUUID,
		}).Count()
	}

	func (ns *mongoModelNamespace) All{{$modelName}}s() {{$storePkg}}.{{$modelName}}Iterator {
		ns.backend.mu.RLock()
		defer ns.backend.mu.RUnlock()

		query := ns.backend.mgoDB.C("{{$modelBackendName}}").Find(bson.M{
			"model-uuid": ns.modelUUID,
		})
		return ns.make{{$modelName}}Iterator(query)
	}

	func (ns *mongoModelNamespace) make{{$modelName}}Iterator(query *mgo.Query) {{$storePkg}}.{{$modelName}}Iterator {
		_, file, line, _ := runtime.Caller(2)
		iteratorID := fmt.Sprintf("%s:%d", file, line)
		ns.openIterators[iteratorID] = struct{}{}

		return &mongo{{$modelName}}Iterator{
			iteratorID: iteratorID,
			ns: ns,
			mgoIt: query.Iter(),
		}
	}

	func (ns *mongoModelNamespace) unmarshal{{$modelName}} (scanner func(interface{}) error) (*{{$modelPkg}}.{{$modelName}}, error){
		var doc = struct {
		{{- range $field := $model.Fields}}
			{{$field.Name.Public}} {{$field.Type.Qualified}} `bson:"{{$field.Name.Backend}}{{- if $field.Flags.IsNullable -}},omitempty{{- end -}}"`
		{{- end }}

		MgoTxnRev int64 `bson:"txn-revno"`
		}{}

		if err := scanner(&doc); err != nil {
			return nil, err
		}

		// Check if we have already emitted this model instance from a previous
		// query; if so return the cached value to guarantee read-your-writes
		// semantics for operations sharing the same ModelNamespace instance.
		{{range $field := $model.Fields}}{{if $field.Flags.IsPK}}
		cacheKey := fmt.Sprintf("{{$modelName}}_%v", doc.{{$field.Name.Public}})
		if inst, hit := ns.modelCache[cacheKey]; hit {
			return inst.(*{{$modelPkg}}.{{$modelName}}), nil
		}
		{{end -}}
		{{end -}}

		inst := {{$modelPkg}}.New{{$modelName}}(doc.MgoTxnRev){{range .Fields -}}.
		Set{{.Name.Public}}(doc.{{.Name.Public}})
		{{- end }}


		// Track instance reference so we can apply any mutations to it when
		// we commit any accumulated changes in the model namespace.
		ns.trackModelReference(inst)
		ns.modelCache[cacheKey] = inst

		inst.ResetModelMutationFlags()
		return inst, nil
	}

	func (ns *mongoModelNamespace) insert{{$modelName}}Op (modelInst *{{$modelPkg}}.{{$modelName}}) txn.Op {
		var doc = bson.M {
			{{range .Fields -}}
				{{- if .Flags.IsPK -}}
				"_id" : makeMongoID(ns.modelUUID, modelInst.{{getter .}}()),
				"model-uuid": ns.modelUUID,
				//
				{{end -}}{{- if not .Flags.IsNullable -}}
				"{{.Name.Backend}}": modelInst.{{getter .}}(),
				{{end -}}
			{{end -}}
		}

		{{range .Fields -}}
			{{- if .Flags.IsNullable -}}
			if val := modelInst.{{getter .}}(); val != nil {
				doc["{{.Name.Backend}}"] = val
			}
			{{end -}}
		{{end}}

		return txn.Op {
			C: "{{$modelBackendName}}",
			Id: 
			{{- range .Fields -}}
				{{- if .Flags.IsPK -}}makeMongoID(ns.modelUUID, modelInst.{{getter .}}()),{{- end -}}
			{{- end }}
			Assert: txn.DocMissing,
			Insert: doc,
		}
	}

	func (ns *mongoModelNamespace) update{{$modelName}}Op (modelInst *{{$modelPkg}}.{{$modelName}}) txn.Op {
		var set, unset = bson.M{}, bson.M{}

		{{range $fieldIndex, $field := .Fields -}}
		if modelInst.IsModelFieldMutated({{$fieldIndex}}){
			{{if not .Flags.IsNullable -}}
			set["{{.Name.Backend}}"] = modelInst.{{getter .}}()
			{{else -}}
			if val := modelInst.{{getter .}}(); val != nil {
				set["{{.Name.Backend}}"] = val
			} else {
				unset["{{.Name.Backend}}"] = 1
			}
			{{end -}}
		}
		{{end -}}

		var updateOps = bson.M{}
		if len(set) != 0 {
			updateOps["$set"] = set
		}
		if len(unset) != 0 {
			updateOps["$unset"] = unset
		}

		return txn.Op {
			C: "{{$modelBackendName}}",
			Id: 
			{{- range .Fields -}}
				{{- if .Flags.IsPK -}}makeMongoID(ns.modelUUID, modelInst.{{getter .}}()),{{- end -}}
			{{- end }}
			Assert: bson.M{"txn-revno": modelInst.ModelVersion()},
			Update: updateOps,
		}
	}

	func (ns *mongoModelNamespace) delete{{$modelName}}Op (modelInst *{{$modelPkg}}.{{$modelName}}) txn.Op {
		return txn.Op {
			C: "{{$modelBackendName}}",
			Id: 
			{{- range .Fields -}}
				{{- if .Flags.IsPK -}}makeMongoID(ns.modelUUID, modelInst.{{getter .}}()),{{- end -}}
			{{- end }}
			Assert: bson.M{"txn-revno": modelInst.ModelVersion()},
			Remove: true,
		}
	}
{{end}}
{{end}}

func (ns *mongoModelNamespace) insertModelOp(modelInst interface{}) (txn.Op, error) {
	switch t := (modelInst).(type){
		{{with nonEmbeddedModels .Models}}
		{{- range .}}
		{{- $modelName := .Name.Public -}}
		case *{{$modelPkg}}.{{$modelName}}:
			return ns.insert{{$modelName}}Op(t), nil
		{{end -}}
		{{end -}}
		default:
			return txn.Op{}, errors.Errorf("unsupported instance type %T", t)
	}
}

func (ns *mongoModelNamespace) updateModelOp(modelInst interface{}) (txn.Op, error) {
	switch t := (modelInst).(type){
		{{with nonEmbeddedModels .Models}}
		{{- range .}}
		{{- $modelName := .Name.Public -}}
		case *{{$modelPkg}}.{{$modelName}}:
			return ns.update{{$modelName}}Op(t), nil
		{{end -}}
		{{end -}}
		default:
			return txn.Op{}, errors.Errorf("unsupported instance type %T", t)
	}
}

func (ns *mongoModelNamespace) deleteModelOp(modelInst interface{}) (txn.Op, error) {
	switch t := (modelInst).(type){
		{{with nonEmbeddedModels .Models}}
		{{- range .}}
		{{- $modelName := .Name.Public -}}
		case *{{$modelPkg}}.{{$modelName}}:
			return ns.delete{{$modelName}}Op(t), nil
		{{end -}}
		{{end -}}
		default:
			return txn.Op{}, errors.Errorf("unsupported instance type %T", t)
	}
}
	
// ModelUUID() returns the Juju model UUID associated with this instance.
func (ns  *mongoModelNamespace) ModelUUID() string {
	return ns.modelUUID		
}

func makeMongoID(modelUUID string, pk interface{}) string {
	return fmt.Sprintf("%s:%s", modelUUID, pk)
}
