package backend

//
// The contents of this file have been auto-generated. Do not modify.
//

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/juju/errors"
	_ "github.com/mattn/go-sqlite3"

	"github.com/achilleasa/jorm/model"
	"github.com/achilleasa/jorm/store"
)

const (
	sqliteQueryList = iota

	sqliteFindApplicationByName
	sqliteFindApplicationsBySeries
	sqliteFindApplicationsByHasResources
	sqliteAllApplications
	sqliteCountApplications
	sqliteDeleteApplication
	sqliteInsertApplication
	sqliteUpdateApplication
	sqliteAssertApplicationVersion

	sqliteFindUnitByName
	sqliteFindUnitsByApplication
	sqliteAllUnits
	sqliteCountUnits
	sqliteDeleteUnit
	sqliteInsertUnit
	sqliteUpdateUnit
	sqliteAssertUnitVersion
)

var _ store.Store = (*SQLite)(nil)

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
		// Table schema for Application models
		`CREATE TABLE IF NOT EXISTS applications ( 
			"name" TEXT, 
			"series" TEXT, 
			"subordinate" INTEGER, 
			"cs-channel" TEXT, 
			"charmmodifiedversion" INTEGER, 
			"forcecharm" INTEGER, 
			"unitcount" INTEGER, 
			"relationcount" INTEGER, 
			"minunits" INTEGER, 
			"metric-credentials" BLOB, 
			"exposed" INTEGER, 
			"exposed-endpoints" TEXT, 
			"scale" INTEGER, 
			"passwordhash" TEXT, 
			"placement" TEXT, 
			"has-resources" INTEGER,
			model_uuid TEXT,
			_version INTEGER,
			PRIMARY KEY(name, model_uuid)
		);
		CREATE INDEX IF NOT EXISTS "idx_modeluuid" ON applications(model_uuid);
		CREATE INDEX IF NOT EXISTS "idx_find_by_series" ON applications(model_uuid, "series");
		CREATE INDEX IF NOT EXISTS "idx_find_by_has-resources" ON applications(model_uuid, "has-resources");
		`,
		// Table schema for Unit models
		`CREATE TABLE IF NOT EXISTS units ( 
			"name" TEXT, 
			"application" TEXT, 
			"series" TEXT, 
			"principal" TEXT, 
			"subordinates" TEXT, 
			"storageattachmentcount" INTEGER, 
			"machineId" TEXT, 
			"passwordHash" TEXT,
			model_uuid TEXT,
			_version INTEGER,
			PRIMARY KEY(name, model_uuid)
		);
		CREATE INDEX IF NOT EXISTS "idx_modeluuid" ON units(model_uuid);
		CREATE INDEX IF NOT EXISTS "idx_find_by_application" ON units(model_uuid, "application");
		`,
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
		// Queries for {Application applications} models.
		sqliteFindApplicationByName:          `SELECT "name","series","subordinate","cs-channel","charmmodifiedversion","forcecharm","unitcount","relationcount","minunits","metric-credentials","exposed","exposed-endpoints","scale","passwordhash","placement","has-resources",_version FROM applications WHERE model_uuid=? AND "name"=?`,
		sqliteFindApplicationsBySeries:       `SELECT "name","series","subordinate","cs-channel","charmmodifiedversion","forcecharm","unitcount","relationcount","minunits","metric-credentials","exposed","exposed-endpoints","scale","passwordhash","placement","has-resources",_version FROM applications WHERE model_uuid=? AND "series"=?`,
		sqliteFindApplicationsByHasResources: `SELECT "name","series","subordinate","cs-channel","charmmodifiedversion","forcecharm","unitcount","relationcount","minunits","metric-credentials","exposed","exposed-endpoints","scale","passwordhash","placement","has-resources",_version FROM applications WHERE model_uuid=? AND "has-resources"=?`,
		sqliteAllApplications:                `SELECT "name","series","subordinate","cs-channel","charmmodifiedversion","forcecharm","unitcount","relationcount","minunits","metric-credentials","exposed","exposed-endpoints","scale","passwordhash","placement","has-resources",_version FROM applications WHERE model_uuid=?`,
		sqliteCountApplications:              `SELECT COUNT(*) FROM applications WHERE model_uuid=?`,
		sqliteDeleteApplication:              `DELETE FROM applications WHERE model_uuid=? AND "name"=?`,
		sqliteInsertApplication:              `INSERT INTO applications ("name","series","subordinate","cs-channel","charmmodifiedversion","forcecharm","unitcount","relationcount","minunits","metric-credentials","exposed","exposed-endpoints","scale","passwordhash","placement","has-resources",model_uuid,_version) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		sqliteUpdateApplication:              `UPDATE applications SET "series"=?,"subordinate"=?,"cs-channel"=?,"charmmodifiedversion"=?,"forcecharm"=?,"unitcount"=?,"relationcount"=?,"minunits"=?,"metric-credentials"=?,"exposed"=?,"exposed-endpoints"=?,"scale"=?,"passwordhash"=?,"placement"=?,"has-resources"=?,_version=_version+1 WHERE model_uuid=? AND "name"=?`,
		sqliteAssertApplicationVersion:       `SELECT 1 FROM applications WHERE model_uuid=? AND "name"=? AND _version=?`,
		// Queries for {Unit units} models.
		sqliteFindUnitByName:         `SELECT "name","application","series","principal","subordinates","storageattachmentcount","machineId","passwordHash",_version FROM units WHERE model_uuid=? AND "name"=?`,
		sqliteFindUnitsByApplication: `SELECT "name","application","series","principal","subordinates","storageattachmentcount","machineId","passwordHash",_version FROM units WHERE model_uuid=? AND "application"=?`,
		sqliteAllUnits:               `SELECT "name","application","series","principal","subordinates","storageattachmentcount","machineId","passwordHash",_version FROM units WHERE model_uuid=?`,
		sqliteCountUnits:             `SELECT COUNT(*) FROM units WHERE model_uuid=?`,
		sqliteDeleteUnit:             `DELETE FROM units WHERE model_uuid=? AND "name"=?`,
		sqliteInsertUnit:             `INSERT INTO units ("name","application","series","principal","subordinates","storageattachmentcount","machineId","passwordHash",model_uuid,_version) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		sqliteUpdateUnit:             `UPDATE units SET "application"=?,"series"=?,"principal"=?,"subordinates"=?,"storageattachmentcount"=?,"machineId"=?,"passwordHash"=?,_version=_version+1 WHERE model_uuid=? AND "name"=?`,
		sqliteAssertUnitVersion:      `SELECT 1 FROM units WHERE model_uuid=? AND "name"=? AND _version=?`,
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
func (s *SQLite) ModelNamespace(modelUUID string) store.ModelNamespace {
	return newSQLiteModelNamespace(modelUUID, s)
}

// ApplyChanges applies changes to models tracked by the specified ModelNamespace.
func (s *SQLite) ApplyChanges(ns store.ModelNamespace) error {
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
		mutTracker, ok := ref.(store.MutationTracker)
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

var _ store.ModelNamespace = (*sqliteModelNamespace)(nil)

type sqliteModelNamespace struct {
	modelUUID string
	backend   *SQLite

	// modelRefs keeps track of all model instances that were obtained via
	// this model namespace instance. The backend queries this information
	// to decide what actions it needs to perform when committing changes.
	modelRefs map[interface{}]struct{}

	// The modelCache serves as an index to any modelRefs that were
	// obtained via the backing store. Each entry uses a key of the form
	// "modelType-PK". When attempting to model.unmarshal a model from the store,
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
	return &sqliteModelNamespace{
		modelUUID:     modelUUID,
		backend:       backend,
		modelRefs:     make(map[interface{}]struct{}),
		modelCache:    make(map[string]interface{}),
		openIterators: make(map[string]struct{}),
	}
}

func (ns *sqliteModelNamespace) trackModelReference(ref interface{}) {
	ns.modelRefs[ref] = struct{}{}
}

//-----------------------------------------
// ApplicationAccessor implementation
//-----------------------------------------

var _ store.ApplicationIterator = (*sqliteApplicationIterator)(nil)

type sqliteApplicationIterator struct {
	iteratorID string
	ns         *sqliteModelNamespace
	rows       *sql.Rows
	cur        *model.Application
	lastErr    error
	closed     bool
}

func (it *sqliteApplicationIterator) Error() error                    { return it.lastErr }
func (it *sqliteApplicationIterator) Application() *model.Application { return it.cur }

func (it *sqliteApplicationIterator) Close() error {
	if it.closed {
		return it.lastErr
	}
	it.closed = true
	it.lastErr = it.rows.Close()
	delete(it.ns.openIterators, it.iteratorID)
	it.ns.backend.mu.RUnlock() // release the read lock once we are done iterating.
	return it.lastErr
}

func (it *sqliteApplicationIterator) Next() bool {
	if it.lastErr != nil {
		return false
	} else if !it.rows.Next() {
		it.lastErr = it.rows.Err()
		return false
	} else if it.cur, it.lastErr = it.ns.unmarshalApplication(it.rows.Scan); it.lastErr != nil {
		return false
	}
	return true
}

func (ns *sqliteModelNamespace) NewApplication() *model.Application {
	inst := model.NewApplication(1)
	ns.trackModelReference(inst) // keep track of the object so it can be inserted.
	return inst
}

func (ns *sqliteModelNamespace) FindApplicationByName(name string) (*model.Application, error) {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()

	// Check if we have already emitted this model instance from a previous
	// query; if so return the cached value to avoid making a query and to
	// provide read-your-writes semantics for operations sharing the same
	// ModelNamespace instance.
	cacheKey := fmt.Sprintf("Application_%v", name)
	if inst, hit := ns.modelCache[cacheKey]; hit {
		return inst.(*model.Application), nil
	}

	row := ns.backend.queries[sqliteFindApplicationByName].QueryRow(ns.modelUUID, name)
	inst, err := ns.unmarshalApplication(row.Scan)
	if err == sql.ErrNoRows {
		return nil, errors.NotFoundf("Application in model %q with Name %q", ns.modelUUID, name)
	}
	return inst, err
}

func (ns *sqliteModelNamespace) FindApplicationsBySeries(series string) store.ApplicationIterator {
	return ns.makeApplicationIterator(ns.backend.queries[sqliteFindApplicationsBySeries], ns.modelUUID, series)
}

func (ns *sqliteModelNamespace) FindApplicationsByHasResources(hasResources bool) store.ApplicationIterator {
	return ns.makeApplicationIterator(ns.backend.queries[sqliteFindApplicationsByHasResources], ns.modelUUID, hasResources)
}

func (ns *sqliteModelNamespace) CountApplications() (int, error) {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()

	var count int
	var err error
	err = ns.backend.queries[sqliteCountApplications].QueryRow(ns.modelUUID).Scan(&count)
	return count, err
}

func (ns *sqliteModelNamespace) AllApplications() store.ApplicationIterator {
	return ns.makeApplicationIterator(ns.backend.queries[sqliteAllApplications], ns.modelUUID)
}

func (ns *sqliteModelNamespace) makeApplicationIterator(stmt *sql.Stmt, args ...interface{}) store.ApplicationIterator {
	_, file, line, _ := runtime.Caller(2)
	iteratorID := fmt.Sprintf("%s:%d", file, line)

	ns.backend.mu.RLock()
	// NOTE: the lock will be released either if an error occurs or when
	// the iterator's Close() method is invoked.

	rows, err := stmt.Query(args...)
	res := &sqliteApplicationIterator{
		iteratorID: iteratorID,
		ns:         ns,
		rows:       rows,
		lastErr:    err,
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

func (ns *sqliteModelNamespace) unmarshalApplication(scanner func(...interface{}) error) (*model.Application, error) {
	var (
		name                 string
		series               string
		subordinate          bool
		channel              string
		charmModifiedVersion int
		forceCharm           bool
		unitCount            int
		relationCount        int
		minUnits             int
		metricCredentials    []byte
		exposed              bool
		exposedEndpointsRaw  []byte
		exposedEndpoints     map[string]model.ExposedEndpoint
		desiredScale         int
		passwordHash         string
		placement            string
		hasResources         bool
		_version             int64
	)

	err := scanner(
		&name,
		&series,
		&subordinate,
		&channel,
		&charmModifiedVersion,
		&forceCharm,
		&unitCount,
		&relationCount,
		&minUnits,
		&metricCredentials,
		&exposed,
		&exposedEndpointsRaw,
		&desiredScale,
		&passwordHash,
		&placement,
		&hasResources,
		&_version,
	)
	if err != nil {
		return nil, err
	}

	// Check if we have already emitted this model instance from a previous
	// query; if so return the cached value to guarantee read-your-writes
	// semantics for operations sharing the same ModelNamespace instance.
	cacheKey := fmt.Sprintf("Application_%v", name)
	if inst, hit := ns.modelCache[cacheKey]; hit {
		return inst.(*model.Application), nil
	}

	if err = json.Unmarshal(exposedEndpointsRaw, &exposedEndpoints); err != nil {
		return nil, errors.Annotate(err, "unmarshaling field exposedEndpoints of Application model")
	}

	inst := model.NewApplication(_version).
		SetName(name).
		SetSeries(series).
		SetSubordinate(subordinate).
		SetChannel(channel).
		SetCharmModifiedVersion(charmModifiedVersion).
		SetForceCharm(forceCharm).
		SetUnitCount(unitCount).
		SetRelationCount(relationCount).
		SetMinUnits(minUnits).
		SetMetricCredentials(metricCredentials).
		SetExposed(exposed).
		SetExposedEndpoints(exposedEndpoints).
		SetDesiredScale(desiredScale).
		SetPasswordHash(passwordHash).
		SetPlacement(placement).
		SetHasResources(hasResources)

	// Track instance reference so we can apply any mutations to it when
	// we commit any accumulated changes in the model namespace.
	ns.trackModelReference(inst)
	ns.modelCache[cacheKey] = inst

	inst.ResetModelMutationFlags()
	return inst, nil
}

func (ns *sqliteModelNamespace) insertApplication(txn *sql.Tx, modelInst *model.Application) error {
	exposedEndpointsRaw, err := json.Marshal(modelInst.GetExposedEndpoints())
	if err != nil {
		return errors.Annotate(err, "marshaling field exposedEndpoints of Application model")
	}

	_, queryErr := txn.Stmt(ns.backend.queries[sqliteInsertApplication]).Exec(modelInst.GetName(),
		modelInst.GetSeries(),
		modelInst.IsSubordinate(),
		modelInst.GetChannel(),
		modelInst.GetCharmModifiedVersion(),
		modelInst.IsForceCharm(),
		modelInst.GetUnitCount(),
		modelInst.GetRelationCount(),
		modelInst.GetMinUnits(),
		modelInst.GetMetricCredentials(),
		modelInst.IsExposed(),
		exposedEndpointsRaw,
		modelInst.GetDesiredScale(),
		modelInst.GetPasswordHash(),
		modelInst.GetPlacement(),
		modelInst.HasResources(),
		ns.modelUUID, // Inserted items are always tagged with the associated model UUID.
		modelInst.ModelVersion(),
	)
	return queryErr
}

func (ns *sqliteModelNamespace) updateApplication(txn *sql.Tx, modelInst *model.Application) error {
	exposedEndpointsRaw, err := json.Marshal(modelInst.GetExposedEndpoints())
	if err != nil {
		return errors.Annotate(err, "marshaling field exposedEndpoints of Application model")
	}

	_, queryErr := txn.Stmt(ns.backend.queries[sqliteUpdateApplication]).Exec(modelInst.GetSeries(),
		modelInst.IsSubordinate(),
		modelInst.GetChannel(),
		modelInst.GetCharmModifiedVersion(),
		modelInst.IsForceCharm(),
		modelInst.GetUnitCount(),
		modelInst.GetRelationCount(),
		modelInst.GetMinUnits(),
		modelInst.GetMetricCredentials(),
		modelInst.IsExposed(),
		exposedEndpointsRaw, modelInst.GetDesiredScale(),
		modelInst.GetPasswordHash(),
		modelInst.GetPlacement(),
		modelInst.HasResources(),
		ns.modelUUID,
		modelInst.GetName(),
	)
	return queryErr
}

func (ns *sqliteModelNamespace) deleteApplication(txn *sql.Tx, modelInst *model.Application) error {
	_, err := txn.Stmt(ns.backend.queries[sqliteDeleteApplication]).Exec(
		ns.modelUUID,
		modelInst.GetName(),
	)
	return err
}

//-----------------------------------------
// UnitAccessor implementation
//-----------------------------------------

var _ store.UnitIterator = (*sqliteUnitIterator)(nil)

type sqliteUnitIterator struct {
	iteratorID string
	ns         *sqliteModelNamespace
	rows       *sql.Rows
	cur        *model.Unit
	lastErr    error
	closed     bool
}

func (it *sqliteUnitIterator) Error() error      { return it.lastErr }
func (it *sqliteUnitIterator) Unit() *model.Unit { return it.cur }

func (it *sqliteUnitIterator) Close() error {
	if it.closed {
		return it.lastErr
	}
	it.closed = true
	it.lastErr = it.rows.Close()
	delete(it.ns.openIterators, it.iteratorID)
	it.ns.backend.mu.RUnlock() // release the read lock once we are done iterating.
	return it.lastErr
}

func (it *sqliteUnitIterator) Next() bool {
	if it.lastErr != nil {
		return false
	} else if !it.rows.Next() {
		it.lastErr = it.rows.Err()
		return false
	} else if it.cur, it.lastErr = it.ns.unmarshalUnit(it.rows.Scan); it.lastErr != nil {
		return false
	}
	return true
}

func (ns *sqliteModelNamespace) NewUnit() *model.Unit {
	inst := model.NewUnit(1)
	ns.trackModelReference(inst) // keep track of the object so it can be inserted.
	return inst
}

func (ns *sqliteModelNamespace) FindUnitByName(name string) (*model.Unit, error) {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()

	// Check if we have already emitted this model instance from a previous
	// query; if so return the cached value to avoid making a query and to
	// provide read-your-writes semantics for operations sharing the same
	// ModelNamespace instance.
	cacheKey := fmt.Sprintf("Unit_%v", name)
	if inst, hit := ns.modelCache[cacheKey]; hit {
		return inst.(*model.Unit), nil
	}

	row := ns.backend.queries[sqliteFindUnitByName].QueryRow(ns.modelUUID, name)
	inst, err := ns.unmarshalUnit(row.Scan)
	if err == sql.ErrNoRows {
		return nil, errors.NotFoundf("Unit in model %q with Name %q", ns.modelUUID, name)
	}
	return inst, err
}

func (ns *sqliteModelNamespace) FindUnitsByApplication(application string) store.UnitIterator {
	return ns.makeUnitIterator(ns.backend.queries[sqliteFindUnitsByApplication], ns.modelUUID, application)
}

func (ns *sqliteModelNamespace) CountUnits() (int, error) {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()

	var count int
	var err error
	err = ns.backend.queries[sqliteCountUnits].QueryRow(ns.modelUUID).Scan(&count)
	return count, err
}

func (ns *sqliteModelNamespace) AllUnits() store.UnitIterator {
	return ns.makeUnitIterator(ns.backend.queries[sqliteAllUnits], ns.modelUUID)
}

func (ns *sqliteModelNamespace) makeUnitIterator(stmt *sql.Stmt, args ...interface{}) store.UnitIterator {
	_, file, line, _ := runtime.Caller(2)
	iteratorID := fmt.Sprintf("%s:%d", file, line)

	ns.backend.mu.RLock()
	// NOTE: the lock will be released either if an error occurs or when
	// the iterator's Close() method is invoked.

	rows, err := stmt.Query(args...)
	res := &sqliteUnitIterator{
		iteratorID: iteratorID,
		ns:         ns,
		rows:       rows,
		lastErr:    err,
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

func (ns *sqliteModelNamespace) unmarshalUnit(scanner func(...interface{}) error) (*model.Unit, error) {
	var (
		name                   string
		application            string
		series                 string
		principal              string
		subordinatesRaw        []byte
		subordinates           []string
		storageAttachmentCount int
		machineId              string
		passwordHash           string
		_version               int64
	)

	err := scanner(
		&name,
		&application,
		&series,
		&principal,
		&subordinatesRaw,
		&storageAttachmentCount,
		&machineId,
		&passwordHash,
		&_version,
	)
	if err != nil {
		return nil, err
	}

	// Check if we have already emitted this model instance from a previous
	// query; if so return the cached value to guarantee read-your-writes
	// semantics for operations sharing the same ModelNamespace instance.
	cacheKey := fmt.Sprintf("Unit_%v", name)
	if inst, hit := ns.modelCache[cacheKey]; hit {
		return inst.(*model.Unit), nil
	}

	if err = json.Unmarshal(subordinatesRaw, &subordinates); err != nil {
		return nil, errors.Annotate(err, "unmarshaling field subordinates of Unit model")
	}

	inst := model.NewUnit(_version).
		SetName(name).
		SetApplication(application).
		SetSeries(series).
		SetPrincipal(principal).
		SetSubordinates(subordinates).
		SetStorageAttachmentCount(storageAttachmentCount).
		SetMachineId(machineId).
		SetPasswordHash(passwordHash)

	// Track instance reference so we can apply any mutations to it when
	// we commit any accumulated changes in the model namespace.
	ns.trackModelReference(inst)
	ns.modelCache[cacheKey] = inst

	inst.ResetModelMutationFlags()
	return inst, nil
}

func (ns *sqliteModelNamespace) insertUnit(txn *sql.Tx, modelInst *model.Unit) error {
	subordinatesRaw, err := json.Marshal(modelInst.GetSubordinates())
	if err != nil {
		return errors.Annotate(err, "marshaling field subordinates of Unit model")
	}

	_, queryErr := txn.Stmt(ns.backend.queries[sqliteInsertUnit]).Exec(modelInst.GetName(),
		modelInst.GetApplication(),
		modelInst.GetSeries(),
		modelInst.GetPrincipal(),
		subordinatesRaw,
		modelInst.GetStorageAttachmentCount(),
		modelInst.GetMachineId(),
		modelInst.GetPasswordHash(),
		ns.modelUUID, // Inserted items are always tagged with the associated model UUID.
		modelInst.ModelVersion(),
	)
	return queryErr
}

func (ns *sqliteModelNamespace) updateUnit(txn *sql.Tx, modelInst *model.Unit) error {
	subordinatesRaw, err := json.Marshal(modelInst.GetSubordinates())
	if err != nil {
		return errors.Annotate(err, "marshaling field subordinates of Unit model")
	}

	_, queryErr := txn.Stmt(ns.backend.queries[sqliteUpdateUnit]).Exec(modelInst.GetApplication(),
		modelInst.GetSeries(),
		modelInst.GetPrincipal(),
		subordinatesRaw, modelInst.GetStorageAttachmentCount(),
		modelInst.GetMachineId(),
		modelInst.GetPasswordHash(),
		ns.modelUUID,
		modelInst.GetName(),
	)
	return queryErr
}

func (ns *sqliteModelNamespace) deleteUnit(txn *sql.Tx, modelInst *model.Unit) error {
	_, err := txn.Stmt(ns.backend.queries[sqliteDeleteUnit]).Exec(
		ns.modelUUID,
		modelInst.GetName(),
	)
	return err
}

// assertModelVersionMatches ensures that the model's version in the store matches
// the version in the specified instance.
//
// This method must be called while holding a read lock in the backing store.
func (ns *sqliteModelNamespace) assertModelVersionMatches(txn *sql.Tx, modelInst interface{}) error {
	var (
		assertQueryID int
		expVersion    int64
		modelPK       interface{}
	)

	switch t := (modelInst).(type) {
	case *model.Application:
		assertQueryID = sqliteAssertApplicationVersion
		expVersion = t.ModelVersion()
		modelPK = t.GetName()
	case *model.Unit:
		assertQueryID = sqliteAssertUnitVersion
		expVersion = t.ModelVersion()
		modelPK = t.GetName()
	default:
		return errors.Errorf("unsupported instance type %T", t)
	}

	var res int
	err := txn.Stmt(ns.backend.queries[assertQueryID]).QueryRow(ns.modelUUID, modelPK, expVersion).Scan(&res)
	if err == sql.ErrNoRows {
		return store.ErrStoreVersionMismatch
	}
	return err
}

func (ns *sqliteModelNamespace) insertModel(txn *sql.Tx, modelInst interface{}) error {
	switch t := (modelInst).(type) {
	case *model.Application:
		return ns.insertApplication(txn, t)
	case *model.Unit:
		return ns.insertUnit(txn, t)
	default:
		return errors.Errorf("unsupported instance type %T", t)
	}
}

func (ns *sqliteModelNamespace) updateModel(txn *sql.Tx, modelInst interface{}) error {
	switch t := (modelInst).(type) {
	case *model.Application:
		return ns.updateApplication(txn, t)
	case *model.Unit:
		return ns.updateUnit(txn, t)
	default:
		return errors.Errorf("unsupported instance type %T", t)
	}
}

func (ns *sqliteModelNamespace) deleteModel(txn *sql.Tx, modelInst interface{}) error {
	switch t := (modelInst).(type) {
	case *model.Application:
		return ns.deleteApplication(txn, t)
	case *model.Unit:
		return ns.deleteUnit(txn, t)
	default:
		return errors.Errorf("unsupported instance type %T", t)
	}
}

// ModelUUID() returns the Juju model UUID associated with this instance.
func (ns *sqliteModelNamespace) ModelUUID() string {
	return ns.modelUUID
}
