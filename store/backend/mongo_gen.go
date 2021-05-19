package backend

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

	"github.com/achilleasa/jorm/model"
	"github.com/achilleasa/jorm/store"
)

var _ store.Store = (*Mongo)(nil)

type txnRunner interface {
	Run(ops []txn.Op, id bson.ObjectId, info interface{}) (err error)
}

// Mongo provides a Store interface implementation that is backed by mongodb.
type Mongo struct {
	mongoURI string
	dbName   string
	useSSTXN bool

	mu         sync.RWMutex
	mgoSession *mgo.Session
	mgoDB      *mgo.Database
}

// NewMongo creates a new Mongo instance backed by dbFile.
func NewMongo(mongoURI, dbName string, useSSTXN bool) *Mongo {
	return &Mongo{
		mongoURI: mongoURI,
		dbName:   dbName,
		useSSTXN: useSSTXN,
	}
}

// Open the store.
func (s *Mongo) Open() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	if s.mgoSession, err = mgo.DialWithTimeout(s.mongoURI, 5*time.Second); err != nil {
		return err
	}

	s.mgoDB = s.mgoSession.DB(s.dbName)
	return s.ensureIndices()
}

// ensureIndices creates the indices for all find_by fields.
func (s *Mongo) ensureIndices() error {
	var col *mgo.Collection

	// Table indices for the Application collection.
	col = s.mgoDB.C("applications")
	if err := col.EnsureIndexKey("model-uuid"); err != nil {
		return errors.Annotate(err, `creating mongo index for field "model-uuid" of collection "applications"`)
	}
	if err := col.EnsureIndexKey("model-uuid", "series"); err != nil {
		return errors.Annotate(err, `creating mongo index for field "series" of collection "applications"`)
	}
	if err := col.EnsureIndexKey("model-uuid", "has-resources"); err != nil {
		return errors.Annotate(err, `creating mongo index for field "has-resources" of collection "applications"`)
	}
	// Table indices for the Unit collection.
	col = s.mgoDB.C("units")
	if err := col.EnsureIndexKey("model-uuid"); err != nil {
		return errors.Annotate(err, `creating mongo index for field "model-uuid" of collection "units"`)
	}
	if err := col.EnsureIndexKey("model-uuid", "application"); err != nil {
		return errors.Annotate(err, `creating mongo index for field "application" of collection "units"`)
	}

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
func (s *Mongo) ModelNamespace(modelUUID string) store.ModelNamespace {
	return newMongoModelNamespace(modelUUID, s)
}

// Mongo provides a Store interface implementation that is backed by mongodb.
func (s *Mongo) ApplyChanges(ns store.ModelNamespace) error {
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
		mutTracker, ok := ref.(store.MutationTracker)
		if !ok {
			return errors.Errorf("bug: ModelNamespace holds a reference to a non-model object")
		}

		var (
			op  txn.Op
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
			return store.ErrStoreVersionMismatch
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

var _ store.ModelNamespace = (*mongoModelNamespace)(nil)

type mongoModelNamespace struct {
	modelUUID string
	backend   *Mongo

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

func newMongoModelNamespace(modelUUID string, backend *Mongo) *mongoModelNamespace {
	return &mongoModelNamespace{
		modelUUID:     modelUUID,
		backend:       backend,
		modelRefs:     make(map[interface{}]struct{}),
		modelCache:    make(map[string]interface{}),
		openIterators: make(map[string]struct{}),
	}
}

func (ns *mongoModelNamespace) trackModelReference(ref interface{}) {
	ns.modelRefs[ref] = struct{}{}
}

//-----------------------------------------
// ApplicationAccessor implementation
//-----------------------------------------

var _ store.ApplicationIterator = (*mongoApplicationIterator)(nil)

type mongoApplicationIterator struct {
	iteratorID string
	ns         *mongoModelNamespace
	mgoIt      *mgo.Iter
	cur        *model.Application
	lastErr    error
	closed     bool
}

func (it *mongoApplicationIterator) Error() error                    { return it.lastErr }
func (it *mongoApplicationIterator) Application() *model.Application { return it.cur }

func (it *mongoApplicationIterator) Close() error {
	if it.closed {
		return it.lastErr
	}
	it.closed = true
	it.lastErr = it.mgoIt.Close()
	delete(it.ns.openIterators, it.iteratorID)
	return it.lastErr
}

func (it *mongoApplicationIterator) Next() bool {
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
	if it.cur, it.lastErr = it.ns.unmarshalApplication(func(dst interface{}) error {
		if !it.mgoIt.Next(dst) {
			return errors.Errorf("Application iterator returned no data even though more data seems to be available")
		}
		return nil
	}); it.lastErr != nil {
		return false
	}
	return true
}

func (ns *mongoModelNamespace) NewApplication() *model.Application {
	inst := model.NewApplication(2) // mgo/txn uses 2 as the initial version.
	ns.trackModelReference(inst)    // keep track of the object so it can be inserted.
	return inst
}

func (ns *mongoModelNamespace) FindApplicationByName(name string) (*model.Application, error) {
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

	query := ns.backend.mgoDB.C("applications").Find(bson.M{"_id": makeMongoID(ns.modelUUID, name)})
	inst, err := ns.unmarshalApplication(query.One)
	if err == mgo.ErrNotFound {
		return nil, errors.NotFoundf("Application in model %q with Name %q", ns.modelUUID, name)
	}
	return inst, err
}

func (ns *mongoModelNamespace) FindApplicationsBySeries(series string) store.ApplicationIterator {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()

	query := ns.backend.mgoDB.C("applications").Find(bson.M{
		"series":     series,
		"model-uuid": ns.modelUUID,
	})
	return ns.makeApplicationIterator(query)
}

func (ns *mongoModelNamespace) FindApplicationsByHasResources(hasResources bool) store.ApplicationIterator {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()

	query := ns.backend.mgoDB.C("applications").Find(bson.M{
		"has-resources": hasResources,
		"model-uuid":    ns.modelUUID,
	})
	return ns.makeApplicationIterator(query)
}

func (ns *mongoModelNamespace) CountApplications() (int, error) {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()

	return ns.backend.mgoDB.C("applications").Find(bson.M{
		"model-uuid": ns.modelUUID,
	}).Count()
}

func (ns *mongoModelNamespace) AllApplications() store.ApplicationIterator {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()

	query := ns.backend.mgoDB.C("applications").Find(bson.M{
		"model-uuid": ns.modelUUID,
	})
	return ns.makeApplicationIterator(query)
}

func (ns *mongoModelNamespace) makeApplicationIterator(query *mgo.Query) store.ApplicationIterator {
	_, file, line, _ := runtime.Caller(2)
	iteratorID := fmt.Sprintf("%s:%d", file, line)
	ns.openIterators[iteratorID] = struct{}{}

	return &mongoApplicationIterator{
		iteratorID: iteratorID,
		ns:         ns,
		mgoIt:      query.Iter(),
	}
}

func (ns *mongoModelNamespace) unmarshalApplication(scanner func(interface{}) error) (*model.Application, error) {
	var doc = struct {
		Name                 string                           `bson:"name"`
		Series               string                           `bson:"series"`
		Subordinate          bool                             `bson:"subordinate"`
		Channel              string                           `bson:"cs-channel"`
		CharmModifiedVersion int                              `bson:"charmmodifiedversion"`
		ForceCharm           bool                             `bson:"forcecharm"`
		UnitCount            int                              `bson:"unitcount"`
		RelationCount        int                              `bson:"relationcount"`
		MinUnits             int                              `bson:"minunits"`
		MetricCredentials    []byte                           `bson:"metric-credentials,omitempty"`
		Exposed              bool                             `bson:"exposed"`
		ExposedEndpoints     map[string]model.ExposedEndpoint `bson:"exposed-endpoints,omitempty"`
		DesiredScale         int                              `bson:"scale"`
		PasswordHash         string                           `bson:"passwordhash"`
		Placement            string                           `bson:"placement"`
		HasResources         bool                             `bson:"has-resources"`

		MgoTxnRev int64 `bson:"txn-revno"`
	}{}

	if err := scanner(&doc); err != nil {
		return nil, err
	}

	// Check if we have already emitted this model instance from a previous
	// query; if so return the cached value to guarantee read-your-writes
	// semantics for operations sharing the same ModelNamespace instance.

	cacheKey := fmt.Sprintf("Application_%v", doc.Name)
	if inst, hit := ns.modelCache[cacheKey]; hit {
		return inst.(*model.Application), nil
	}
	inst := model.NewApplication(doc.MgoTxnRev).
		SetName(doc.Name).
		SetSeries(doc.Series).
		SetSubordinate(doc.Subordinate).
		SetChannel(doc.Channel).
		SetCharmModifiedVersion(doc.CharmModifiedVersion).
		SetForceCharm(doc.ForceCharm).
		SetUnitCount(doc.UnitCount).
		SetRelationCount(doc.RelationCount).
		SetMinUnits(doc.MinUnits).
		SetMetricCredentials(doc.MetricCredentials).
		SetExposed(doc.Exposed).
		SetExposedEndpoints(doc.ExposedEndpoints).
		SetDesiredScale(doc.DesiredScale).
		SetPasswordHash(doc.PasswordHash).
		SetPlacement(doc.Placement).
		SetHasResources(doc.HasResources)

	// Track instance reference so we can apply any mutations to it when
	// we commit any accumulated changes in the model namespace.
	ns.trackModelReference(inst)
	ns.modelCache[cacheKey] = inst

	inst.ResetModelMutationFlags()
	return inst, nil
}

func (ns *mongoModelNamespace) insertApplicationOp(modelInst *model.Application) txn.Op {
	var doc = bson.M{
		"_id":        makeMongoID(ns.modelUUID, modelInst.GetName()),
		"model-uuid": ns.modelUUID,
		//
		"name":                 modelInst.GetName(),
		"series":               modelInst.GetSeries(),
		"subordinate":          modelInst.IsSubordinate(),
		"cs-channel":           modelInst.GetChannel(),
		"charmmodifiedversion": modelInst.GetCharmModifiedVersion(),
		"forcecharm":           modelInst.IsForceCharm(),
		"unitcount":            modelInst.GetUnitCount(),
		"relationcount":        modelInst.GetRelationCount(),
		"minunits":             modelInst.GetMinUnits(),
		"exposed":              modelInst.IsExposed(),
		"scale":                modelInst.GetDesiredScale(),
		"passwordhash":         modelInst.GetPasswordHash(),
		"placement":            modelInst.GetPlacement(),
		"has-resources":        modelInst.HasResources(),
	}

	if val := modelInst.GetMetricCredentials(); val != nil {
		doc["metric-credentials"] = val
	}
	if val := modelInst.GetExposedEndpoints(); val != nil {
		doc["exposed-endpoints"] = val
	}

	return txn.Op{
		C:      "applications",
		Id:     makeMongoID(ns.modelUUID, modelInst.GetName()),
		Assert: txn.DocMissing,
		Insert: doc,
	}
}

func (ns *mongoModelNamespace) updateApplicationOp(modelInst *model.Application) txn.Op {
	var set, unset = bson.M{}, bson.M{}

	if modelInst.IsModelFieldMutated(0) {
		set["name"] = modelInst.GetName()
	}
	if modelInst.IsModelFieldMutated(1) {
		set["series"] = modelInst.GetSeries()
	}
	if modelInst.IsModelFieldMutated(2) {
		set["subordinate"] = modelInst.IsSubordinate()
	}
	if modelInst.IsModelFieldMutated(3) {
		set["cs-channel"] = modelInst.GetChannel()
	}
	if modelInst.IsModelFieldMutated(4) {
		set["charmmodifiedversion"] = modelInst.GetCharmModifiedVersion()
	}
	if modelInst.IsModelFieldMutated(5) {
		set["forcecharm"] = modelInst.IsForceCharm()
	}
	if modelInst.IsModelFieldMutated(6) {
		set["unitcount"] = modelInst.GetUnitCount()
	}
	if modelInst.IsModelFieldMutated(7) {
		set["relationcount"] = modelInst.GetRelationCount()
	}
	if modelInst.IsModelFieldMutated(8) {
		set["minunits"] = modelInst.GetMinUnits()
	}
	if modelInst.IsModelFieldMutated(9) {
		if val := modelInst.GetMetricCredentials(); val != nil {
			set["metric-credentials"] = val
		} else {
			unset["metric-credentials"] = 1
		}
	}
	if modelInst.IsModelFieldMutated(10) {
		set["exposed"] = modelInst.IsExposed()
	}
	if modelInst.IsModelFieldMutated(11) {
		if val := modelInst.GetExposedEndpoints(); val != nil {
			set["exposed-endpoints"] = val
		} else {
			unset["exposed-endpoints"] = 1
		}
	}
	if modelInst.IsModelFieldMutated(12) {
		set["scale"] = modelInst.GetDesiredScale()
	}
	if modelInst.IsModelFieldMutated(13) {
		set["passwordhash"] = modelInst.GetPasswordHash()
	}
	if modelInst.IsModelFieldMutated(14) {
		set["placement"] = modelInst.GetPlacement()
	}
	if modelInst.IsModelFieldMutated(15) {
		set["has-resources"] = modelInst.HasResources()
	}
	var updateOps = bson.M{}
	if len(set) != 0 {
		updateOps["$set"] = set
	}
	if len(unset) != 0 {
		updateOps["$unset"] = unset
	}

	return txn.Op{
		C:      "applications",
		Id:     makeMongoID(ns.modelUUID, modelInst.GetName()),
		Assert: bson.M{"txn-revno": modelInst.ModelVersion()},
		Update: updateOps,
	}
}

func (ns *mongoModelNamespace) deleteApplicationOp(modelInst *model.Application) txn.Op {
	return txn.Op{
		C:      "applications",
		Id:     makeMongoID(ns.modelUUID, modelInst.GetName()),
		Assert: bson.M{"txn-revno": modelInst.ModelVersion()},
		Remove: true,
	}
}

//-----------------------------------------
// UnitAccessor implementation
//-----------------------------------------

var _ store.UnitIterator = (*mongoUnitIterator)(nil)

type mongoUnitIterator struct {
	iteratorID string
	ns         *mongoModelNamespace
	mgoIt      *mgo.Iter
	cur        *model.Unit
	lastErr    error
	closed     bool
}

func (it *mongoUnitIterator) Error() error      { return it.lastErr }
func (it *mongoUnitIterator) Unit() *model.Unit { return it.cur }

func (it *mongoUnitIterator) Close() error {
	if it.closed {
		return it.lastErr
	}
	it.closed = true
	it.lastErr = it.mgoIt.Close()
	delete(it.ns.openIterators, it.iteratorID)
	return it.lastErr
}

func (it *mongoUnitIterator) Next() bool {
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
	if it.cur, it.lastErr = it.ns.unmarshalUnit(func(dst interface{}) error {
		if !it.mgoIt.Next(dst) {
			return errors.Errorf("Unit iterator returned no data even though more data seems to be available")
		}
		return nil
	}); it.lastErr != nil {
		return false
	}
	return true
}

func (ns *mongoModelNamespace) NewUnit() *model.Unit {
	inst := model.NewUnit(2)     // mgo/txn uses 2 as the initial version.
	ns.trackModelReference(inst) // keep track of the object so it can be inserted.
	return inst
}

func (ns *mongoModelNamespace) FindUnitByName(name string) (*model.Unit, error) {
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

	query := ns.backend.mgoDB.C("units").Find(bson.M{"_id": makeMongoID(ns.modelUUID, name)})
	inst, err := ns.unmarshalUnit(query.One)
	if err == mgo.ErrNotFound {
		return nil, errors.NotFoundf("Unit in model %q with Name %q", ns.modelUUID, name)
	}
	return inst, err
}

func (ns *mongoModelNamespace) FindUnitsByApplication(application string) store.UnitIterator {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()

	query := ns.backend.mgoDB.C("units").Find(bson.M{
		"application": application,
		"model-uuid":  ns.modelUUID,
	})
	return ns.makeUnitIterator(query)
}

func (ns *mongoModelNamespace) CountUnits() (int, error) {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()

	return ns.backend.mgoDB.C("units").Find(bson.M{
		"model-uuid": ns.modelUUID,
	}).Count()
}

func (ns *mongoModelNamespace) AllUnits() store.UnitIterator {
	ns.backend.mu.RLock()
	defer ns.backend.mu.RUnlock()

	query := ns.backend.mgoDB.C("units").Find(bson.M{
		"model-uuid": ns.modelUUID,
	})
	return ns.makeUnitIterator(query)
}

func (ns *mongoModelNamespace) makeUnitIterator(query *mgo.Query) store.UnitIterator {
	_, file, line, _ := runtime.Caller(2)
	iteratorID := fmt.Sprintf("%s:%d", file, line)
	ns.openIterators[iteratorID] = struct{}{}

	return &mongoUnitIterator{
		iteratorID: iteratorID,
		ns:         ns,
		mgoIt:      query.Iter(),
	}
}

func (ns *mongoModelNamespace) unmarshalUnit(scanner func(interface{}) error) (*model.Unit, error) {
	var doc = struct {
		Name                   string   `bson:"name"`
		Application            string   `bson:"application"`
		Series                 string   `bson:"series"`
		Principal              string   `bson:"principal"`
		Subordinates           []string `bson:"subordinates,omitempty"`
		StorageAttachmentCount int      `bson:"storageattachmentcount"`
		MachineId              string   `bson:"machineId"`
		PasswordHash           string   `bson:"passwordHash"`

		MgoTxnRev int64 `bson:"txn-revno"`
	}{}

	if err := scanner(&doc); err != nil {
		return nil, err
	}

	// Check if we have already emitted this model instance from a previous
	// query; if so return the cached value to guarantee read-your-writes
	// semantics for operations sharing the same ModelNamespace instance.

	cacheKey := fmt.Sprintf("Unit_%v", doc.Name)
	if inst, hit := ns.modelCache[cacheKey]; hit {
		return inst.(*model.Unit), nil
	}
	inst := model.NewUnit(doc.MgoTxnRev).
		SetName(doc.Name).
		SetApplication(doc.Application).
		SetSeries(doc.Series).
		SetPrincipal(doc.Principal).
		SetSubordinates(doc.Subordinates).
		SetStorageAttachmentCount(doc.StorageAttachmentCount).
		SetMachineId(doc.MachineId).
		SetPasswordHash(doc.PasswordHash)

	// Track instance reference so we can apply any mutations to it when
	// we commit any accumulated changes in the model namespace.
	ns.trackModelReference(inst)
	ns.modelCache[cacheKey] = inst

	inst.ResetModelMutationFlags()
	return inst, nil
}

func (ns *mongoModelNamespace) insertUnitOp(modelInst *model.Unit) txn.Op {
	var doc = bson.M{
		"_id":        makeMongoID(ns.modelUUID, modelInst.GetName()),
		"model-uuid": ns.modelUUID,
		//
		"name":                   modelInst.GetName(),
		"application":            modelInst.GetApplication(),
		"series":                 modelInst.GetSeries(),
		"principal":              modelInst.GetPrincipal(),
		"storageattachmentcount": modelInst.GetStorageAttachmentCount(),
		"machineId":              modelInst.GetMachineId(),
		"passwordHash":           modelInst.GetPasswordHash(),
	}

	if val := modelInst.GetSubordinates(); val != nil {
		doc["subordinates"] = val
	}

	return txn.Op{
		C:      "units",
		Id:     makeMongoID(ns.modelUUID, modelInst.GetName()),
		Assert: txn.DocMissing,
		Insert: doc,
	}
}

func (ns *mongoModelNamespace) updateUnitOp(modelInst *model.Unit) txn.Op {
	var set, unset = bson.M{}, bson.M{}

	if modelInst.IsModelFieldMutated(0) {
		set["name"] = modelInst.GetName()
	}
	if modelInst.IsModelFieldMutated(1) {
		set["application"] = modelInst.GetApplication()
	}
	if modelInst.IsModelFieldMutated(2) {
		set["series"] = modelInst.GetSeries()
	}
	if modelInst.IsModelFieldMutated(3) {
		set["principal"] = modelInst.GetPrincipal()
	}
	if modelInst.IsModelFieldMutated(4) {
		if val := modelInst.GetSubordinates(); val != nil {
			set["subordinates"] = val
		} else {
			unset["subordinates"] = 1
		}
	}
	if modelInst.IsModelFieldMutated(5) {
		set["storageattachmentcount"] = modelInst.GetStorageAttachmentCount()
	}
	if modelInst.IsModelFieldMutated(6) {
		set["machineId"] = modelInst.GetMachineId()
	}
	if modelInst.IsModelFieldMutated(7) {
		set["passwordHash"] = modelInst.GetPasswordHash()
	}
	var updateOps = bson.M{}
	if len(set) != 0 {
		updateOps["$set"] = set
	}
	if len(unset) != 0 {
		updateOps["$unset"] = unset
	}

	return txn.Op{
		C:      "units",
		Id:     makeMongoID(ns.modelUUID, modelInst.GetName()),
		Assert: bson.M{"txn-revno": modelInst.ModelVersion()},
		Update: updateOps,
	}
}

func (ns *mongoModelNamespace) deleteUnitOp(modelInst *model.Unit) txn.Op {
	return txn.Op{
		C:      "units",
		Id:     makeMongoID(ns.modelUUID, modelInst.GetName()),
		Assert: bson.M{"txn-revno": modelInst.ModelVersion()},
		Remove: true,
	}
}

func (ns *mongoModelNamespace) insertModelOp(modelInst interface{}) (txn.Op, error) {
	switch t := (modelInst).(type) {
	case *model.Application:
		return ns.insertApplicationOp(t), nil
	case *model.Unit:
		return ns.insertUnitOp(t), nil
	default:
		return txn.Op{}, errors.Errorf("unsupported instance type %T", t)
	}
}

func (ns *mongoModelNamespace) updateModelOp(modelInst interface{}) (txn.Op, error) {
	switch t := (modelInst).(type) {
	case *model.Application:
		return ns.updateApplicationOp(t), nil
	case *model.Unit:
		return ns.updateUnitOp(t), nil
	default:
		return txn.Op{}, errors.Errorf("unsupported instance type %T", t)
	}
}

func (ns *mongoModelNamespace) deleteModelOp(modelInst interface{}) (txn.Op, error) {
	switch t := (modelInst).(type) {
	case *model.Application:
		return ns.deleteApplicationOp(t), nil
	case *model.Unit:
		return ns.deleteUnitOp(t), nil
	default:
		return txn.Op{}, errors.Errorf("unsupported instance type %T", t)
	}
}

// ModelUUID() returns the Juju model UUID associated with this instance.
func (ns *mongoModelNamespace) ModelUUID() string {
	return ns.modelUUID
}

func makeMongoID(modelUUID string, pk interface{}) string {
	return fmt.Sprintf("%s:%s", modelUUID, pk)
}
