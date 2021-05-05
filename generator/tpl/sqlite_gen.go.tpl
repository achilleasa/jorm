package {{basePkgName .TargetPkg}}

//
// The contents of this file have been auto-generated. Do not modify.
//

import (
	"database/sql"
	{{if (sqlModelsNeedSerialization .Models)}}"encoding/json"{{end}}
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/juju/errors"
	_ "github.com/mattn/go-sqlite3"
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

const (
	sqliteQueryList = iota
{{with nonEmbeddedModels .Models}}
{{range .}}
	{{$model := .}}
	{{$modelName := .Name.Public}}
	{{- range $model.Fields -}}
		{{- if .Flags.IsPK}}
			sqliteFind{{$modelName}}By{{.Name.Public}}
		{{- else if .Flags.IsFindBy}}
			sqliteFind{{$modelName}}sBy{{.Name.Public}}
		{{- end -}}
	{{end}}
	sqliteAll{{$modelName}}s
	sqliteCount{{$modelName}}s
	sqliteDelete{{$modelName}}
	sqliteInsert{{$modelName}}
	sqliteUpdate{{$modelName}}
	sqliteAssert{{$modelName}}Version
{{end}}
{{end}}
)

var _ {{$storePkg}}.Store = (*SQLite)(nil)

// SQLite provides a Store interface implementation that is backed by sqlite.
type SQLite struct {
	dbFile  string
	queries map[int]*sql.Stmt

	mu sync.RWMutex
	db *sql.DB
}

// NewSQLite creates a new SQLite instance backed by dbFile.
func NewSQLite(dbFile string) *SQLite {
	return &SQLite{
		dbFile: dbFile,
	}
}

// Open the store.
func (s *SQLite) Open() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	if s.db, err = sql.Open("sqlite3", s.dbFile); err != nil {
		return err
	}

	if err = s.ensureSchema(); err != nil {
		return errors.Annotate(err, "unable to apply schema changes")
	} else if err = s.prepareQueries(); err != nil {
		return errors.Annotate(err, "unable to prepare sql statements")
	}

	return nil
}

// ensureSchema applies the DB schema to the DB. It is expected that the caller
// is holding the write lock on the SQLite instance when calling this method.
func (s *SQLite) ensureSchema() error {
	var tableSchemas = []string{
{{with nonEmbeddedModels .Models}}
{{- range . -}}
	{{$model := .}}
	{{- $modelName := .Name.Public -}}
		// Table schema for {{$modelName}} models
		`CREATE TABLE IF NOT EXISTS {{.Name.Backend}} (
			{{- range $model.Fields}} 
			"{{.Name.Backend}}" {{sqliteFieldType .Type.Resolved}},
			{{- end}}
			model_uuid TEXT,
			_version INTEGER,
			PRIMARY KEY({{pkFieldName .}}, model_uuid)
		);
		CREATE INDEX IF NOT EXISTS "idx_modeluuid" ON {{$model.Name.Backend}}(model_uuid);
		{{range $field := $model.Fields -}}{{- if $field.Flags.IsFindBy -}}
		CREATE INDEX IF NOT EXISTS "idx_find_by_{{$field.Name.Backend}}" ON {{$model.Name.Backend}}(model_uuid, "{{$field.Name.Backend}}");
		{{end -}}
		{{end -}}`,
{{end}}
{{end}}
	}

	txn, err := s.db.Begin()
	if err != nil {
		return err
	}

	for _, schemaDef := range tableSchemas {
		if _, err := txn.Exec(schemaDef); err != nil {
			_ = txn.Rollback()
			return err
		}
	}

	return txn.Commit()
}

// prepareQueries is invoked by the SQLite backend at initialization time so 
// that all model-related queries can be prepared. It is expected that the 
// caller is holding the write lock on the SQLite instance when calling this
// method.
func (s *SQLite) prepareQueries() error {
	rawQueries := map[int]string{
{{with nonEmbeddedModels .Models}}
{{- range . -}}
	{{- $model := . -}}
	{{- $modelName := .Name.Public -}}
	// Queries for {{.Name}} models.
	{{- range $field := $model.Fields -}}
		{{- if $field.Flags.IsPK}}
			sqliteFind{{$modelName}}By{{.Name.Public}} : `SELECT {{sqlFieldNameList $model}},_version FROM {{$model.Name.Backend}} WHERE model_uuid=? AND "{{$field.Name.Backend}}"=?`,
		{{- else if $field.Flags.IsFindBy}}
			sqliteFind{{$modelName}}sBy{{.Name.Public}} : `SELECT {{sqlFieldNameList $model}},_version FROM {{$model.Name.Backend}} WHERE model_uuid=? AND "{{$field.Name.Backend}}"=?`,
		{{- end -}}
	{{end}}
	sqliteAll{{$modelName}}s : `SELECT {{sqlFieldNameList $model}},_version FROM {{$model.Name.Backend}} WHERE model_uuid=?`,
	sqliteCount{{$modelName}}s : `SELECT COUNT(*) FROM {{$model.Name.Backend}} WHERE model_uuid=?`,
	sqliteDelete{{$modelName}} : `DELETE FROM {{$model.Name.Backend}} WHERE model_uuid=? AND "{{pkFieldName $model}}"=?`,
	sqliteInsert{{$modelName}} : `INSERT INTO {{$model.Name.Backend}} ({{sqlFieldNameList $model}},model_uuid,_version) VALUES ({{sqlInsertPlaceholderList $model}},?,?)`,
	sqliteUpdate{{$modelName}} : `UPDATE {{$model.Name.Backend}} SET {{sqlUpdatePlaceholderList $model}},_version=_version+1 WHERE model_uuid=? AND "{{pkFieldName $model}}"=?`,
	sqliteAssert{{$modelName}}Version : `SELECT 1 FROM {{$model.Name.Backend}} WHERE model_uuid=? AND "{{pkFieldName $model}}"=? AND _version=?`,
{{end}}
{{end}}
	}
	
	s.queries = make(map[int]*sql.Stmt, len(rawQueries))
	for queryID, rawQuery := range rawQueries {
		stmt, err := s.db.Prepare(rawQuery)
		if err != nil {
			return err
		}

		s.queries[queryID] = stmt
	}
	return nil
}

// Close the store.
func (s *SQLite) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		return nil
	}

	err := s.db.Close()
	s.db = nil
	return err
}

// ModelNamespace returns a ModelNamespace instance scoped to modelUUID.
func (s *SQLite) ModelNamespace(modelUUID string) {{$storePkg}}.ModelNamespace {
	return newSQLiteModelNamespace(modelUUID, s)
}

// ApplyChanges applies changes to models tracked by the specified ModelNamespace.
func (s *SQLite) ApplyChanges(ns {{$storePkg}}.ModelNamespace) error {
	sqliteNS, ok := ns.(*sqliteModelNamespace)
	if !ok {
		return errors.Errorf("unexpected ModelNamespace instance")
	}

	// Check for any leaked iterators
	if len(sqliteNS.openIterators) > 0 {
		var callerList []string
		for caller := range sqliteNS.openIterators {
			callerList = append(callerList, caller)
		}

		return errors.Errorf("unable to apply changes as the following iterators have not been closed:\n%s", strings.Join(callerList, "\n"))
	}

	// When mutating the DB, always acquire the write lock. SQLite uses a
	// single writer / multiple readers model.
	s.mu.Lock()
	defer s.mu.Unlock()

	txn, err := s.db.Begin()
	if err != nil {
		return err
	}

	for ref := range sqliteNS.modelRefs {
		// Assert versions for any read/mutated/deleted item before
		// applying changes.
		mutTracker, ok := ref.({{$storePkg}}.MutationTracker)
		if !ok {
			_ = txn.Rollback()
			return errors.Errorf("bug: ModelNamespace holds a reference to a non-model object")
		}
		if mutTracker.IsModelPersisted() && (mutTracker.IsModelMutated() || mutTracker.IsModelDeleted()) {
			if err := sqliteNS.assertModelVersionMatches(txn, ref); err != nil {
				_ = txn.Rollback()
				return err
			}
		}

		switch {
		case mutTracker.IsModelDeleted():
			if err = sqliteNS.deleteModel(txn, ref); err != nil {
				err = errors.Annotate(err, "deleting model")
			}
		case !mutTracker.IsModelPersisted():
			if err = sqliteNS.insertModel(txn, ref); err != nil {
				err = errors.Annotate(err, "inserting model")
			}
		case mutTracker.IsModelMutated():
			if err = sqliteNS.updateModel(txn, ref); err != nil {
				err = errors.Annotate(err, "update model")
			}
		}

		if err != nil {
			_ = txn.Rollback()
			return err
		}
	}

	return txn.Commit()
}

var _ {{$storePkg}}.ModelNamespace = (*sqliteModelNamespace)(nil)

type sqliteModelNamespace struct {
	modelUUID string
	backend *SQLite

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

func newSQLiteModelNamespace(modelUUID string, backend *SQLite) *sqliteModelNamespace {
	return &sqliteModelNamespace {
		modelUUID: modelUUID,
		backend: backend,
		modelRefs: make(map[interface{}]struct{}),
		modelCache: make(map[string]interface{}),
		openIterators: make(map[string]struct{}),
	}
}

func (ns *sqliteModelNamespace) trackModelReference(ref interface{}) {
	ns.modelRefs[ref] = struct{}{}
}

{{$models := .Models}}
{{with nonEmbeddedModels .Models}}
{{range .}}
	{{$model := .}}
	{{$modelName := .Name.Public}}
	//-----------------------------------------
	// {{$modelName}}Accessor implementation
	//-----------------------------------------

	var _ {{$storePkg}}.{{$modelName}}Iterator = (*sqlite{{$modelName}}Iterator)(nil)

	type sqlite{{$modelName}}Iterator struct {
		iteratorID string
		ns *sqliteModelNamespace
		rows *sql.Rows
		cur *{{$modelPkg}}.{{$modelName}}
		lastErr error
		closed bool
	}

	func (it *sqlite{{$modelName}}Iterator) Error() error { return it.lastErr }
	func (it *sqlite{{$modelName}}Iterator) {{$modelName}}() *{{$modelPkg}}.{{$modelName}} { return it.cur }

	func (it *sqlite{{$modelName}}Iterator) Close() error { 
		if it.closed {
			return it.lastErr
		}
		it.closed = true
		it.lastErr = it.rows.Close() 
		delete(it.ns.openIterators, it.iteratorID)
		it.ns.backend.mu.RUnlock() // release the read lock once we are done iterating.
		return it.lastErr
	}

	func (it *sqlite{{$modelName}}Iterator) Next() bool {
		if it.lastErr != nil {
			return false
		} else if !it.rows.Next() {
			it.lastErr = it.rows.Err()
			return false
		} else if it.cur, it.lastErr = it.ns.unmarshal{{$modelName}}(it.rows.Scan); it.lastErr != nil {
			return false
		}
		return true
	}

	func (ns *sqliteModelNamespace) New{{$modelName}}() *{{$modelPkg}}.{{$modelName}} {
		inst := {{$modelPkg}}.New{{$modelName}}(1)
		ns.trackModelReference(inst) // keep track of the object so it can be inserted.
		return inst
	}

	{{range $field := $model.Fields}}
		{{if .Flags.IsPK}}
		func (ns *sqliteModelNamespace) Find{{$modelName}}By{{$field.Name.Public}}({{$field.Name.Private}} {{$field.Type.Qualified}}) (*{{$modelPkg}}.{{$modelName}}, error) {
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

			row  := ns.backend.queries[sqliteFind{{$modelName}}By{{$field.Name.Public}}].QueryRow(ns.modelUUID, {{$field.Name.Private}})
			inst, err := ns.unmarshal{{$modelName}}(row.Scan)
			if err == sql.ErrNoRows {
				return nil, errors.NotFoundf("{{$modelName}} in model %q with {{$field.Name.Public}} %q", ns.modelUUID, {{$field.Name.Private}})
			}
			return inst, err
		}
		{{else if .Flags.IsFindBy}}
		func (ns *sqliteModelNamespace) Find{{$modelName}}sBy{{$field.Name.Public}}({{$field.Name.Private}} {{$field.Type.Qualified}}) {{$storePkg}}.{{$modelName}}Iterator {
			return ns.make{{$modelName}}Iterator(ns.backend.queries[sqliteFind{{$modelName}}sBy{{$field.Name.Public}}], ns.modelUUID, {{$field.Name.Private}})
		}
		{{end}}
	{{end}}

	func (ns *sqliteModelNamespace) Count{{$modelName}}s() (int, error) {
		ns.backend.mu.RLock()
		defer ns.backend.mu.RUnlock()

		var count int
		var err error
		err = ns.backend.queries[sqliteCount{{$modelName}}s].QueryRow(ns.modelUUID).Scan(&count)
		return count, err
	}

	func (ns *sqliteModelNamespace) All{{$modelName}}s() {{$storePkg}}.{{$modelName}}Iterator {
		return ns.make{{$modelName}}Iterator(ns.backend.queries[sqliteAll{{$modelName}}s], ns.modelUUID)
	}

	func (ns *sqliteModelNamespace) make{{$modelName}}Iterator(stmt *sql.Stmt, args ...interface{}) {{$storePkg}}.{{$modelName}}Iterator {
		_, file, line, _ := runtime.Caller(2)
		iteratorID := fmt.Sprintf("%s:%d", file, line)

		ns.backend.mu.RLock()
		// NOTE: the lock will be released either if an error occurs or when
		// the iterator's Close() method is invoked.

		rows, err := stmt.Query(args...)
		res := &sqlite{{$modelName}}Iterator{
			iteratorID: iteratorID,
			ns: ns,
			rows: rows,
			lastErr: err, 
		}

		// If an error occurred, release the lock (nothing to read) and mark
		// the iterator as closed so no attempt will be made to release the 
		// lock if Close() is invoked.
		if err != nil {
			ns.backend.mu.RUnlock()
			res.closed = true 
		}
		ns.openIterators[iteratorID] = struct{}{}
		return res
	}

	func (ns *sqliteModelNamespace) unmarshal{{$modelName}} (scanner func(...interface{}) error) (*{{$modelPkg}}.{{$modelName}}, error){
		var (
		{{- range $model.Fields}}
			{{if sqlSerializeField .Type.Resolved -}}
				{{.Name.Private}}Raw []byte
			{{end -}}
			{{.Name.Private}} {{.Type.Qualified}}
		{{- end }}
			_version int64
		)

		err := scanner(
		{{- range $model.Fields}}
			&{{.Name.Private}}{{if sqlSerializeField .Type.Resolved -}}Raw{{end}},
		{{- end }}
			&_version,
		)
		if err != nil {
			return nil, err
		}

		// Check if we have already emitted this model instance from a previous
		// query; if so return the cached value to guarantee read-your-writes
		// semantics for operations sharing the same ModelNamespace instance.
		cacheKey := fmt.Sprintf("{{$modelName}}_%v", {{pkFieldName $model}})
		if inst, hit := ns.modelCache[cacheKey]; hit {
			return inst.(*{{$modelPkg}}.{{$modelName}}), nil
		}

		{{range .Fields}}
		{{- if sqlSerializeField .Type.Resolved}}
		if err = json.Unmarshal({{.Name.Private}}Raw, &{{.Name.Private}}); err != nil {
			return nil, errors.Annotate(err, "unmarshaling field {{.Name.Private}} of {{$modelName}} model")
		}
		{{- end -}}
		{{- end}}
		
		inst := {{$modelPkg}}.New{{$modelName}}(_version){{range .Fields -}}.
		Set{{.Name.Public}}({{.Name.Private}})
		{{- end }}

		// Track instance reference so we can apply any mutations to it when
		// we commit any accumulated changes in the model namespace.
		ns.trackModelReference(inst)
		ns.modelCache[cacheKey] = inst

		inst.ResetModelMutationFlags()
		return inst, nil
	}

	func (ns *sqliteModelNamespace) insert{{$modelName}} (txn *sql.Tx, modelInst *{{$modelPkg}}.{{$modelName}}) error {
		{{- range .Fields}}{{if sqlSerializeField .Type.Resolved}}
		{{.Name.Private}}Raw, err := json.Marshal(modelInst.{{getter .}}())
		if err != nil {
			return errors.Annotate(err, "marshaling field {{.Name.Private}} of {{$modelName}} model")
		}

		{{end}}{{end}}
		_, queryErr := txn.Stmt(ns.backend.queries[sqliteInsert{{$modelName}}]).Exec(
			{{- range .Fields -}}
			{{- if sqlSerializeField .Type.Resolved -}}{{.Name.Private}}Raw,
			{{- else -}}modelInst.{{getter .}}(),{{end}}
			{{end -}}
			ns.modelUUID, // Inserted items are always tagged with the associated model UUID. 
			modelInst.ModelVersion(),
		)
		return queryErr
	}

	func (ns *sqliteModelNamespace) update{{$modelName}} (txn *sql.Tx, modelInst *{{$modelPkg}}.{{$modelName}}) error {
		{{- range .Fields}}{{if sqlSerializeField .Type.Resolved}}
		{{.Name.Private}}Raw, err := json.Marshal(modelInst.{{getter .}}())
		if err != nil {
			return errors.Annotate(err, "marshaling field {{.Name.Private}} of {{$modelName}} model")
		}

		{{end}}{{end}}
		_, queryErr := txn.Stmt(ns.backend.queries[sqliteUpdate{{$modelName}}]).Exec(
			{{- range .Fields -}}
			{{if not .Flags.IsPK}}
				{{- if sqlSerializeField .Type.Resolved -}}{{.Name.Private}}Raw,
				{{- else -}}modelInst.{{getter .}}(),
				{{end -}}
			{{end -}}
			{{end -}}
			ns.modelUUID,
			{{range .Fields -}}
			{{if .Flags.IsPK}}modelInst.{{getter .}}(),{{end}}
			{{end -}}
		)
		return queryErr
	}

	func (ns *sqliteModelNamespace) delete{{$modelName}} (txn *sql.Tx, modelInst *{{$modelPkg}}.{{$modelName}}) error {
		_, err := txn.Stmt(ns.backend.queries[sqliteDelete{{$modelName}}]).Exec(
			ns.modelUUID,
			{{range .Fields -}}{{if .Flags.IsPK}}modelInst.{{getter .}}(),{{end}}
			{{end -}}
		)
		return err
	}
{{end}}
{{end}}

// assertModelVersionMatches ensures that the model's version in the store matches
// the version in the specified instance.
//
// This method must be called while holding a read lock in the backing store.
func (ns *sqliteModelNamespace) assertModelVersionMatches(txn *sql.Tx, modelInst interface{}) error {
	var (
		assertQueryID int
		expVersion int64
		modelPK interface{}
	)

	switch t := (modelInst).(type){
		{{- with nonEmbeddedModels .Models -}}
		{{range .}}
		{{$modelName := .Name.Public -}}
		case *{{$modelPkg}}.{{$modelName}}:
			assertQueryID = sqliteAssert{{$modelName}}Version
			expVersion = t.ModelVersion()
			{{range .Fields -}}{{if .Flags.IsPK}}modelPK = t.{{getter .}}(){{end}}{{end -}}
		{{end -}}
		{{end}}
		default:
			return errors.Errorf("unsupported instance type %T", t)
	}

	var res int
	err := txn.Stmt(ns.backend.queries[assertQueryID]).QueryRow(ns.modelUUID, modelPK, expVersion).Scan(&res)
	if err == sql.ErrNoRows {
		return {{$storePkg}}.ErrStoreVersionMismatch
	}
	return err
}

func (ns *sqliteModelNamespace) insertModel(txn *sql.Tx, modelInst interface{}) error {
	switch t := (modelInst).(type){
		{{with nonEmbeddedModels .Models}}
		{{- range .}}
		{{- $modelName := .Name.Public -}}
		case *{{$modelPkg}}.{{$modelName}}:
			return ns.insert{{$modelName}}(txn, t)
		{{end -}}
		{{end -}}
		default:
			return errors.Errorf("unsupported instance type %T", t)
	}
}

func (ns *sqliteModelNamespace) updateModel(txn *sql.Tx, modelInst interface{}) error {
	switch t := (modelInst).(type){
		{{with nonEmbeddedModels .Models}}
		{{- range .}}
		{{- $modelName := .Name.Public -}}
		case *{{$modelPkg}}.{{$modelName}}:
			return ns.update{{$modelName}}(txn, t)
		{{end -}}
		{{end -}}
		default:
			return errors.Errorf("unsupported instance type %T", t)
	}
}

func (ns *sqliteModelNamespace) deleteModel(txn *sql.Tx, modelInst interface{}) error {
	switch t := (modelInst).(type){
		{{with nonEmbeddedModels .Models}}
		{{- range .}}
		{{- $modelName := .Name.Public -}}
		case *{{$modelPkg}}.{{$modelName}}:
			return ns.delete{{$modelName}}(txn, t)
		{{end -}}
		{{end -}}
		default:
			return errors.Errorf("unsupported instance type %T", t)
	}
}
	
// ModelUUID() returns the Juju model UUID associated with this instance.
func (ns  *sqliteModelNamespace) ModelUUID() string {
	return ns.modelUUID		
}
