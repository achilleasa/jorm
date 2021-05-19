package backend

//
// The contents of this file have been auto-generated. Do not modify.
//

import (
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/juju/errors"

	"github.com/achilleasa/jorm/model"
	"github.com/achilleasa/jorm/store"
)

var _ store.Store = (*InMemory)(nil)

// inMemoryDB stores all model collections for a model UUID. It stores all data
// in memory using a map per model type.
type inMemoryDB struct {
	mu                     sync.RWMutex
	applicationsCollection map[string]*model.Application
	unitsCollection        map[string]*model.Unit
}

func newInMemoryDB() *inMemoryDB {
	return &inMemoryDB{
		applicationsCollection: make(map[string]*model.Application),
		unitsCollection:        make(map[string]*model.Unit),
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
	s.db = make(map[string]*inMemoryDB)
	return nil
}

// Close the store.
func (s *InMemory) Close() error {
	return nil
}

// ModelNamespace returns a ModelNamespace instance scoped to modelUUID.
func (s *InMemory) ModelNamespace(modelUUID string) store.ModelNamespace {
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
func (s *InMemory) ApplyChanges(ns store.ModelNamespace) error {
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
		mutTracker, ok := ref.(store.MutationTracker)
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
		mutTracker := ref.(store.MutationTracker)
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

var _ store.ModelNamespace = (*inMemModelNamespace)(nil)

type inMemModelNamespace struct {
	modelUUID string
	backend   *inMemoryDB

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

func newInMemoryModelNamespace(modelUUID string, backend *inMemoryDB) *inMemModelNamespace {
	return &inMemModelNamespace{
		modelUUID:     modelUUID,
		backend:       backend,
		modelRefs:     make(map[interface{}]struct{}),
		modelCache:    make(map[string]interface{}),
		openIterators: make(map[string]struct{}),
	}
}

func (ns *inMemModelNamespace) trackModelReference(ref interface{}) {
	ns.modelRefs[ref] = struct{}{}
}

//-----------------------------------------
// ApplicationAccessor implementation
//-----------------------------------------

var _ store.ApplicationIterator = (*inMemApplicationIterator)(nil)

type inMemApplicationIterator struct {
	iteratorID string
	ns         *inMemModelNamespace
	rows       []*model.Application // cloned copies off the in-mem DB
	rowIndex   int
	cur        *model.Application
	closed     bool
}

func (it *inMemApplicationIterator) Error() error                    { return nil }
func (it *inMemApplicationIterator) Application() *model.Application { return it.cur }

func (it *inMemApplicationIterator) Close() error {
	if it.closed {
		return nil
	}
	it.closed = true
	delete(it.ns.openIterators, it.iteratorID)
	return nil
}

func (it *inMemApplicationIterator) Next() bool {
	if it.rowIndex >= len(it.rows)-1 {
		return false
	}

	it.rowIndex++
	it.cur = it.rows[it.rowIndex]
	return true
}

func (ns *inMemModelNamespace) NewApplication() *model.Application {
	inst := model.NewApplication(1)
	ns.trackModelReference(inst) // keep track of the object so it can be inserted.
	return inst
}

func (ns *inMemModelNamespace) cloneApplication(inst *model.Application) *model.Application {
	// Check if we have already emitted this model instance from a previous
	// query; if so return the cached value to guarantee read-your-writes
	// semantics for operations sharing the same ModelNamespace instance.
	cacheKey := fmt.Sprintf("Application_%v", inst.GetName())
	if inst, hit := ns.modelCache[cacheKey]; hit {
		return inst.(*model.Application)
	}

	clone := model.NewApplication(inst.ModelVersion())
	ns.copyApplication(inst, clone)

	// Track instance reference so we can apply any mutations to it when
	// we commit any accumulated changes in the model namespace.
	ns.trackModelReference(clone)
	ns.modelCache[cacheKey] = clone
	return clone
}

func (ns *inMemModelNamespace) copyApplication(src, dst *model.Application) {
	dst.
		SetName(src.GetName()).
		SetSeries(src.GetSeries()).
		SetSubordinate(src.IsSubordinate()).
		SetChannel(src.GetChannel()).
		SetCharmModifiedVersion(src.GetCharmModifiedVersion()).
		SetForceCharm(src.IsForceCharm()).
		SetUnitCount(src.GetUnitCount()).
		SetRelationCount(src.GetRelationCount()).
		SetMinUnits(src.GetMinUnits()).
		SetMetricCredentials(src.GetMetricCredentials()).
		SetExposed(src.IsExposed()).
		SetExposedEndpoints(src.GetExposedEndpoints()).
		SetDesiredScale(src.GetDesiredScale()).
		SetPasswordHash(src.GetPasswordHash()).
		SetPlacement(src.GetPlacement()).
		SetHasResources(src.HasResources())
	dst.ResetModelMutationFlags()
}

func (ns *inMemModelNamespace) FindApplicationByName(name string) (*model.Application, error) {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()

	inst, found := ns.backend.applicationsCollection[fmt.Sprint(name)]
	if !found {
		return nil, errors.NotFoundf("Application in model %q with Name %q", ns.modelUUID, name)
	}

	// Clone and cache item.
	return ns.cloneApplication(inst), nil
}

func (ns *inMemModelNamespace) FindApplicationsBySeries(series string) store.ApplicationIterator {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()

	var rows []*model.Application
	for _, inst := range ns.backend.applicationsCollection {
		if inst.GetSeries() != series {
			continue
		}
		// Clone and cache item before appending to the result list.
		rows = append(rows, ns.cloneApplication(inst))
	}
	return ns.makeApplicationIterator(rows)
}

func (ns *inMemModelNamespace) FindApplicationsByHasResources(hasResources bool) store.ApplicationIterator {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()

	var rows []*model.Application
	for _, inst := range ns.backend.applicationsCollection {
		if inst.HasResources() != hasResources {
			continue
		}
		// Clone and cache item before appending to the result list.
		rows = append(rows, ns.cloneApplication(inst))
	}
	return ns.makeApplicationIterator(rows)
}

func (ns *inMemModelNamespace) CountApplications() (int, error) {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()
	return len(ns.backend.applicationsCollection), nil
}

func (ns *inMemModelNamespace) AllApplications() store.ApplicationIterator {
	var rows []*model.Application
	for _, inst := range ns.backend.applicationsCollection {
		// Clone and cache item before appending to the result list.
		rows = append(rows, ns.cloneApplication(inst))
	}
	return ns.makeApplicationIterator(rows)
}

func (ns *inMemModelNamespace) makeApplicationIterator(rows []*model.Application) store.ApplicationIterator {
	_, file, line, _ := runtime.Caller(2)
	iteratorID := fmt.Sprintf("%s:%d", file, line)

	res := &inMemApplicationIterator{
		iteratorID: iteratorID,
		ns:         ns,
		rows:       rows,
		rowIndex:   -1,
	}

	ns.openIterators[iteratorID] = struct{}{}
	return res
}

//-----------------------------------------
// UnitAccessor implementation
//-----------------------------------------

var _ store.UnitIterator = (*inMemUnitIterator)(nil)

type inMemUnitIterator struct {
	iteratorID string
	ns         *inMemModelNamespace
	rows       []*model.Unit // cloned copies off the in-mem DB
	rowIndex   int
	cur        *model.Unit
	closed     bool
}

func (it *inMemUnitIterator) Error() error      { return nil }
func (it *inMemUnitIterator) Unit() *model.Unit { return it.cur }

func (it *inMemUnitIterator) Close() error {
	if it.closed {
		return nil
	}
	it.closed = true
	delete(it.ns.openIterators, it.iteratorID)
	return nil
}

func (it *inMemUnitIterator) Next() bool {
	if it.rowIndex >= len(it.rows)-1 {
		return false
	}

	it.rowIndex++
	it.cur = it.rows[it.rowIndex]
	return true
}

func (ns *inMemModelNamespace) NewUnit() *model.Unit {
	inst := model.NewUnit(1)
	ns.trackModelReference(inst) // keep track of the object so it can be inserted.
	return inst
}

func (ns *inMemModelNamespace) cloneUnit(inst *model.Unit) *model.Unit {
	// Check if we have already emitted this model instance from a previous
	// query; if so return the cached value to guarantee read-your-writes
	// semantics for operations sharing the same ModelNamespace instance.
	cacheKey := fmt.Sprintf("Unit_%v", inst.GetName())
	if inst, hit := ns.modelCache[cacheKey]; hit {
		return inst.(*model.Unit)
	}

	clone := model.NewUnit(inst.ModelVersion())
	ns.copyUnit(inst, clone)

	// Track instance reference so we can apply any mutations to it when
	// we commit any accumulated changes in the model namespace.
	ns.trackModelReference(clone)
	ns.modelCache[cacheKey] = clone
	return clone
}

func (ns *inMemModelNamespace) copyUnit(src, dst *model.Unit) {
	dst.
		SetName(src.GetName()).
		SetApplication(src.GetApplication()).
		SetSeries(src.GetSeries()).
		SetPrincipal(src.GetPrincipal()).
		SetSubordinates(src.GetSubordinates()).
		SetStorageAttachmentCount(src.GetStorageAttachmentCount()).
		SetMachineId(src.GetMachineId()).
		SetPasswordHash(src.GetPasswordHash())
	dst.ResetModelMutationFlags()
}

func (ns *inMemModelNamespace) FindUnitByName(name string) (*model.Unit, error) {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()

	inst, found := ns.backend.unitsCollection[fmt.Sprint(name)]
	if !found {
		return nil, errors.NotFoundf("Unit in model %q with Name %q", ns.modelUUID, name)
	}

	// Clone and cache item.
	return ns.cloneUnit(inst), nil
}

func (ns *inMemModelNamespace) FindUnitsByApplication(application string) store.UnitIterator {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()

	var rows []*model.Unit
	for _, inst := range ns.backend.unitsCollection {
		if inst.GetApplication() != application {
			continue
		}
		// Clone and cache item before appending to the result list.
		rows = append(rows, ns.cloneUnit(inst))
	}
	return ns.makeUnitIterator(rows)
}

func (ns *inMemModelNamespace) CountUnits() (int, error) {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()
	return len(ns.backend.unitsCollection), nil
}

func (ns *inMemModelNamespace) AllUnits() store.UnitIterator {
	var rows []*model.Unit
	for _, inst := range ns.backend.unitsCollection {
		// Clone and cache item before appending to the result list.
		rows = append(rows, ns.cloneUnit(inst))
	}
	return ns.makeUnitIterator(rows)
}

func (ns *inMemModelNamespace) makeUnitIterator(rows []*model.Unit) store.UnitIterator {
	_, file, line, _ := runtime.Caller(2)
	iteratorID := fmt.Sprintf("%s:%d", file, line)

	res := &inMemUnitIterator{
		iteratorID: iteratorID,
		ns:         ns,
		rows:       rows,
		rowIndex:   -1,
	}

	ns.openIterators[iteratorID] = struct{}{}
	return res
}

// assertModelVersionMatches ensures that the model's version in the store matches
// the version in the specified instance.
//
// This method must be called while holding a read lock in the backing store.
func (ns *inMemModelNamespace) assertModelVersionMatches(modelInst interface{}) error {
	var (
		instVersion  int64
		storeVersion int64
	)

	switch t := (modelInst).(type) {
	case *model.Application:
		instVersion = t.ModelVersion()
		if stInst, found := ns.backend.applicationsCollection[fmt.Sprint(t.GetName())]; found {
			storeVersion = stInst.ModelVersion()
		}
	case *model.Unit:
		instVersion = t.ModelVersion()
		if stInst, found := ns.backend.unitsCollection[fmt.Sprint(t.GetName())]; found {
			storeVersion = stInst.ModelVersion()
		}
	default:
		return errors.Errorf("unsupported instance type %T", t)
	}

	if instVersion != storeVersion {
		return store.ErrStoreVersionMismatch
	}
	return nil
}

func (ns *inMemModelNamespace) assertModelHasUniquePK(modelInst interface{}) error {
	var pkExists bool
	switch t := (modelInst).(type) {
	case *model.Application:
		if _, pkExists = ns.backend.applicationsCollection[fmt.Sprint(t.GetName())]; pkExists {
			return errors.Errorf("PK key violation for Application:  PK %v already exists", t.GetName())
		}
	case *model.Unit:
		if _, pkExists = ns.backend.unitsCollection[fmt.Sprint(t.GetName())]; pkExists {
			return errors.Errorf("PK key violation for Unit:  PK %v already exists", t.GetName())
		}
	}
	// PK is unique
	return nil
}

func (ns *inMemModelNamespace) insertModel(modelInst interface{}) {
	// insertModel assumes the caller has already checked for PK violations.
	switch t := (modelInst).(type) {
	case *model.Application:
		clone := model.NewApplication(t.ModelVersion())
		ns.copyApplication(t, clone)
		ns.backend.applicationsCollection[fmt.Sprint(clone.GetName())] = clone
	case *model.Unit:
		clone := model.NewUnit(t.ModelVersion())
		ns.copyUnit(t, clone)
		ns.backend.unitsCollection[fmt.Sprint(clone.GetName())] = clone
	}
}

func (ns *inMemModelNamespace) updateModel(modelInst interface{}) {
	switch t := (modelInst).(type) {
	case *model.Application:
		clone := model.NewApplication(t.ModelVersion() + 1) // bump version
		ns.copyApplication(t, clone)
		ns.backend.applicationsCollection[fmt.Sprint(clone.GetName())] = clone
	case *model.Unit:
		clone := model.NewUnit(t.ModelVersion() + 1) // bump version
		ns.copyUnit(t, clone)
		ns.backend.unitsCollection[fmt.Sprint(clone.GetName())] = clone
	}
}

func (ns *inMemModelNamespace) deleteModel(modelInst interface{}) {
	switch t := (modelInst).(type) {
	case *model.Application:
		delete(ns.backend.applicationsCollection, fmt.Sprint(t.GetName()))
	case *model.Unit:
		delete(ns.backend.unitsCollection, fmt.Sprint(t.GetName()))
	}
}

// ModelUUID() returns the Juju model UUID associated with this instance.
func (ns *inMemModelNamespace) ModelUUID() string {
	return ns.modelUUID
}
