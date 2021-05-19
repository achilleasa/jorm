package storetest

//
// The contents of this file have been auto-generated. Do not modify.
//

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"unicode"

	gc "gopkg.in/check.v1"

	"github.com/achilleasa/jorm/model"
	"github.com/achilleasa/jorm/store"
)

const (
	checkVersion      = true
	doNotCheckVersion = false
)

// StoreBaseSuite implements a store-agnostic test-suite harness which allows
// testing the behavior of model-store interactions with any supported store
// implementations.
type StoreBaseSuite struct {
	// A Store instance which the embedding test suite provides via
	// ConfigureSuite().
	st store.Store

	// A cleanup function to truncate the store contents which the embedding
	// test suite provides via ConfigureSuite().
	cleanupFunc func(*gc.C)
}

// Configure the suite parameters. This method must be called before running
// any of the test suite methods or an error will be raised.
func (s *StoreBaseSuite) Configure(st store.Store, cleanupFunc func(*gc.C)) {
	s.st = st
	s.cleanupFunc = cleanupFunc
}

func (s *StoreBaseSuite) SetUpTest(c *gc.C) {
	c.Assert(s.st, gc.Not(gc.IsNil), gc.Commentf("Configure() must be called before running any tests in this suite"))
	c.Assert(s.cleanupFunc, gc.Not(gc.IsNil), gc.Commentf("Configure() must be called before running any tests in this suite"))
	s.cleanupFunc(c)

	// Ensure each test run uses the same generated data.
	rand.Seed(42)
}

func (s *StoreBaseSuite) TestInsertApplication(c *gc.C) {
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"

	// Create and persist a new Application instance
	ns := s.st.ModelNamespace(modelUUID)
	c.Assert(ns.ModelUUID(), gc.Equals, modelUUID, gc.Commentf("the ModelNamespace implementation does not correctly track the associated modelUUID"))
	inst := ns.NewApplication()
	populateFakeData(inst)
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Lookup persisted Application using a new model namespace
	ns = s.st.ModelNamespace(modelUUID)

	got, err := ns.FindApplicationByName(inst.GetName())
	c.Assert(err, gc.IsNil)

	s.assertApplicationsEqual(c, inst, got, checkVersion)
}

func (s *StoreBaseSuite) TestInsertApplicationWithNilFields(c *gc.C) {
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"

	// Create and persist a new Application instance
	ns := s.st.ModelNamespace(modelUUID)
	c.Assert(ns.ModelUUID(), gc.Equals, modelUUID, gc.Commentf("the ModelNamespace implementation does not correctly track the associated modelUUID"))
	inst := ns.NewApplication()
	populateFakeData(inst)

	// Clear nullable fields
	inst.
		SetMetricCredentials(nil).
		SetExposedEndpoints(nil)

	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Lookup persisted Application using a new model namespace
	ns = s.st.ModelNamespace(modelUUID)

	got, err := ns.FindApplicationByName(inst.GetName())
	c.Assert(err, gc.IsNil)

	s.assertApplicationsEqual(c, inst, got, checkVersion)
}

func (s *StoreBaseSuite) TestFindApplicationCacheSupport(c *gc.C) {
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"

	// Create and persist a new Application instance
	ns := s.st.ModelNamespace(modelUUID)
	inst := ns.NewApplication()
	populateFakeData(inst)
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Lookup persisted Application twice using a new model namespace.
	// Then, check that the second lookup returned the same object instance
	// (pointer) as the first.
	ns = s.st.ModelNamespace(modelUUID)

	got1, err := ns.FindApplicationByName(inst.GetName())
	c.Assert(err, gc.IsNil)

	got2, err := ns.FindApplicationByName(inst.GetName())
	c.Assert(err, gc.IsNil)

	c.Assert(got1, gc.Equals, got2, gc.Commentf("expected both lookups to yield the same Application pointer due to caching"))
}

func (s *StoreBaseSuite) TestUpdateApplication(c *gc.C) {
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"

	// Create and persist a new Application instance
	ns := s.st.ModelNamespace(modelUUID)
	origInst := ns.NewApplication()
	populateFakeData(origInst)
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Lookup persisted Application and randomize its fields (but retain the PK field)
	ns = s.st.ModelNamespace(modelUUID)
	gotInserted, err := ns.FindApplicationByName(origInst.GetName())
	c.Assert(err, gc.IsNil)
	populateFakeData(gotInserted)
	gotInserted.SetName(origInst.GetName()) // retain original PK
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Now lookup the updated model and verify that the fields match
	ns = s.st.ModelNamespace(modelUUID)
	gotUpdated, err := ns.FindApplicationByName(origInst.GetName())
	c.Assert(err, gc.IsNil)

	c.Assert(gotUpdated.ModelVersion(), gc.Equals, gotInserted.ModelVersion()+1, gc.Commentf("expected model version to be bumped after update"))
	s.assertApplicationsEqual(c, gotInserted, gotUpdated, doNotCheckVersion)
}

func (s *StoreBaseSuite) TestAbortApplicationUpdateIfVersionMismatch(c *gc.C) {
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"

	// Create and persist a new Application instance
	ns := s.st.ModelNamespace(modelUUID)
	origInst := ns.NewApplication()
	populateFakeData(origInst)
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Create two separate namespaces that both try to mutate the same
	// model at the same time.
	nsA := s.st.ModelNamespace(modelUUID)
	instA, err := nsA.FindApplicationByName(origInst.GetName())
	c.Assert(err, gc.IsNil)
	populateFakeData(instA)
	instA.SetName(origInst.GetName()) // retain original PK

	nsB := s.st.ModelNamespace(modelUUID)
	instB, err := nsB.FindApplicationByName(origInst.GetName())
	c.Assert(err, gc.IsNil)
	populateFakeData(instB)
	instB.SetName(origInst.GetName()) // retain original PK

	// Commit instB changes first
	c.Assert(s.st.ApplyChanges(nsB), gc.IsNil)

	// Trying to commit instA changes should fail as the model version in
	// the store has been bumped after successfully committing the instB changes.
	err = s.st.ApplyChanges(nsA)
	c.Assert(err, gc.Equals, store.ErrStoreVersionMismatch, gc.Commentf("expected update to fail as the model version in the store has changed"))
}

func (s *StoreBaseSuite) TestDeleteApplication(c *gc.C) {
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"

	// Create and persist a new Application instance
	ns := s.st.ModelNamespace(modelUUID)
	inst := ns.NewApplication()
	populateFakeData(inst)
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Check count
	count, err := ns.CountApplications()
	c.Assert(err, gc.IsNil)
	c.Assert(count, gc.Equals, 1, gc.Commentf("expected CountApplication() to return 1"))

	// Lookup persisted Application using a new model namespace and delete it
	ns = s.st.ModelNamespace(modelUUID)
	c.Assert(err, gc.IsNil)

	got, err := ns.FindApplicationByName(inst.GetName())
	c.Assert(err, gc.IsNil)

	got.Delete()
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Check count after deletion
	count, err = ns.CountApplications()
	c.Assert(err, gc.IsNil)
	c.Assert(count, gc.Equals, 0, gc.Commentf("expected CountApplication() to return 0"))
}

func (s *StoreBaseSuite) TestNamespaceSeparationForApplications(c *gc.C) {
	// Create and persist a new Application instance scoped to modelUUID1
	modelUUID1 := "3a92d636-a8b8-11eb-bcbc-0242ac130002"
	ns1 := s.st.ModelNamespace(modelUUID1)
	inst1 := ns1.NewApplication()
	populateFakeData(inst1)
	c.Assert(s.st.ApplyChanges(ns1), gc.IsNil)

	// Create and persist a new Application instance scoped to modelUUID2
	modelUUID2 := "99999999-a8b8-11eb-bcbc-0242ac130002"
	ns2 := s.st.ModelNamespace(modelUUID2)
	inst2 := ns2.NewApplication()
	populateFakeData(inst2)
	c.Assert(s.st.ApplyChanges(ns2), gc.IsNil)

	// Iterate all Application instances scoped to modelUUID1 and ensure we
	// only get back inst1.
	ns1 = s.st.ModelNamespace(modelUUID1)
	instList := s.consumeApplicationIterator(c, ns1.AllApplications())
	c.Assert(instList, gc.HasLen, 1, gc.Commentf("expected to get back only the models in NS1"))

	// Iterate all Application instances scoped to modelUUID2 and ensure we
	// only get back inst2.
	ns2 = s.st.ModelNamespace(modelUUID2)
	instList = s.consumeApplicationIterator(c, ns2.AllApplications())
	c.Assert(instList, gc.HasLen, 1, gc.Commentf("expected to get back only the models in NS2"))
}

func (s *StoreBaseSuite) TestFindApplicationsBySeries(c *gc.C) {
	numInstances := 10

	// Create and persist a batch of Application instances with the same series.
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"
	ns := s.st.ModelNamespace(modelUUID)

	var searchFieldTemplate string
	searchFieldVal := genScalarValue(reflect.TypeOf(searchFieldTemplate)).(string)

	for i := 0; i < numInstances; i++ {
		inst := ns.NewApplication()
		populateFakeData(inst)
		inst.SetSeries(searchFieldVal)
	}
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Use a new namespace to run the search query.
	ns = s.st.ModelNamespace(modelUUID)
	it := ns.FindApplicationsBySeries(searchFieldVal)
	instList := s.consumeApplicationIterator(c, it)
	c.Assert(instList, gc.HasLen, numInstances, gc.Commentf("expected FindApplicationsBySeries() to return %d instances", numInstances))
}

func (s *StoreBaseSuite) TestFindApplicationsByHasResources(c *gc.C) {
	numInstances := 10

	// Create and persist a batch of Application instances with the same hasResources.
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"
	ns := s.st.ModelNamespace(modelUUID)

	var searchFieldTemplate bool
	searchFieldVal := genScalarValue(reflect.TypeOf(searchFieldTemplate)).(bool)

	for i := 0; i < numInstances; i++ {
		inst := ns.NewApplication()
		populateFakeData(inst)
		inst.SetHasResources(searchFieldVal)
	}
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Use a new namespace to run the search query.
	ns = s.st.ModelNamespace(modelUUID)
	it := ns.FindApplicationsByHasResources(searchFieldVal)
	instList := s.consumeApplicationIterator(c, it)
	c.Assert(instList, gc.HasLen, numInstances, gc.Commentf("expected FindApplicationsByHasResources() to return %d instances", numInstances))
}

func (s *StoreBaseSuite) TestApplicationIteratorUsesCache(c *gc.C) {
	numInstances := 10

	// Create and persist a batch of Application instances.
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"
	ns := s.st.ModelNamespace(modelUUID)
	for i := 0; i < numInstances; i++ {
		inst := ns.NewApplication()
		populateFakeData(inst)
	}
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Use a new namespace to run the same search query twice. Due to the
	// use of the cache, both iterators must yield the same Application
	// pointers.
	ns = s.st.ModelNamespace(modelUUID)
	instList1 := s.consumeApplicationIterator(c, ns.AllApplications())
	instList2 := s.consumeApplicationIterator(c, ns.AllApplications())
	// As the iterators may yield results in different order, sort by PK
	// before comparing the individual entries.
	sort.Slice(instList1, func(i, j int) bool { return instList1[i].GetName() < instList1[j].GetName() })
	sort.Slice(instList2, func(i, j int) bool { return instList2[i].GetName() < instList2[j].GetName() })
	c.Assert(len(instList1), gc.Equals, len(instList2))
	for i := 0; i < len(instList1); i++ {
		c.Assert(instList1[i], gc.Equals, instList2[i], gc.Commentf("expected both iterators to yield the same Application pointers due to caching"))
	}
}

func (s *StoreBaseSuite) TestApplicationIteratorLeakDetection(c *gc.C) {
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"
	ns := s.st.ModelNamespace(modelUUID)

	// Get an iterator but call ApplyChanges without closing it first.
	it := ns.AllApplications()
	defer it.Close()
	err := s.st.ApplyChanges(ns)

	c.Assert(err, gc.ErrorMatches, "(?m).*unable to apply changes as the following iterators have not been closed.*")
}

func (s *StoreBaseSuite) assertApplicationsEqual(c *gc.C, a, b *model.Application, checkVersions bool) {
	c.Assert(a.GetName(), gc.DeepEquals, b.GetName(), gc.Commentf("value mismatch for field: name"))
	c.Assert(a.GetSeries(), gc.DeepEquals, b.GetSeries(), gc.Commentf("value mismatch for field: series"))
	c.Assert(a.IsSubordinate(), gc.DeepEquals, b.IsSubordinate(), gc.Commentf("value mismatch for field: subordinate"))
	c.Assert(a.GetChannel(), gc.DeepEquals, b.GetChannel(), gc.Commentf("value mismatch for field: channel"))
	c.Assert(a.GetCharmModifiedVersion(), gc.DeepEquals, b.GetCharmModifiedVersion(), gc.Commentf("value mismatch for field: charmModifiedVersion"))
	c.Assert(a.IsForceCharm(), gc.DeepEquals, b.IsForceCharm(), gc.Commentf("value mismatch for field: forceCharm"))
	c.Assert(a.GetUnitCount(), gc.DeepEquals, b.GetUnitCount(), gc.Commentf("value mismatch for field: unitCount"))
	c.Assert(a.GetRelationCount(), gc.DeepEquals, b.GetRelationCount(), gc.Commentf("value mismatch for field: relationCount"))
	c.Assert(a.GetMinUnits(), gc.DeepEquals, b.GetMinUnits(), gc.Commentf("value mismatch for field: minUnits"))
	c.Assert(a.GetMetricCredentials(), gc.DeepEquals, b.GetMetricCredentials(), gc.Commentf("value mismatch for field: metricCredentials"))
	c.Assert(a.IsExposed(), gc.DeepEquals, b.IsExposed(), gc.Commentf("value mismatch for field: exposed"))
	c.Assert(a.GetExposedEndpoints(), gc.DeepEquals, b.GetExposedEndpoints(), gc.Commentf("value mismatch for field: exposedEndpoints"))
	c.Assert(a.GetDesiredScale(), gc.DeepEquals, b.GetDesiredScale(), gc.Commentf("value mismatch for field: desiredScale"))
	c.Assert(a.GetPasswordHash(), gc.DeepEquals, b.GetPasswordHash(), gc.Commentf("value mismatch for field: passwordHash"))
	c.Assert(a.GetPlacement(), gc.DeepEquals, b.GetPlacement(), gc.Commentf("value mismatch for field: placement"))
	c.Assert(a.HasResources(), gc.DeepEquals, b.HasResources(), gc.Commentf("value mismatch for field: hasResources"))

	if checkVersions {
		c.Assert(a.ModelVersion(), gc.Equals, b.ModelVersion(), gc.Commentf("version mismatch"))
	}
}

func (s *StoreBaseSuite) consumeApplicationIterator(c *gc.C, it store.ApplicationIterator) []*model.Application {
	defer func() {
		// Ensure iterator is always closed even if we get an error.
		it.Close()
	}()
	var res []*model.Application
	for it.Next() {
		res = append(res, it.Application())
	}
	c.Assert(it.Error(), gc.IsNil, gc.Commentf("encountered error while iterating Applications"))
	c.Assert(it.Close(), gc.IsNil, gc.Commentf("encountered error while closing Application iterator"))
	return res
}
func (s *StoreBaseSuite) TestInsertUnit(c *gc.C) {
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"

	// Create and persist a new Unit instance
	ns := s.st.ModelNamespace(modelUUID)
	c.Assert(ns.ModelUUID(), gc.Equals, modelUUID, gc.Commentf("the ModelNamespace implementation does not correctly track the associated modelUUID"))
	inst := ns.NewUnit()
	populateFakeData(inst)
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Lookup persisted Unit using a new model namespace
	ns = s.st.ModelNamespace(modelUUID)

	got, err := ns.FindUnitByName(inst.GetName())
	c.Assert(err, gc.IsNil)

	s.assertUnitsEqual(c, inst, got, checkVersion)
}

func (s *StoreBaseSuite) TestInsertUnitWithNilFields(c *gc.C) {
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"

	// Create and persist a new Unit instance
	ns := s.st.ModelNamespace(modelUUID)
	c.Assert(ns.ModelUUID(), gc.Equals, modelUUID, gc.Commentf("the ModelNamespace implementation does not correctly track the associated modelUUID"))
	inst := ns.NewUnit()
	populateFakeData(inst)

	// Clear nullable fields
	inst.
		SetSubordinates(nil)

	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Lookup persisted Unit using a new model namespace
	ns = s.st.ModelNamespace(modelUUID)

	got, err := ns.FindUnitByName(inst.GetName())
	c.Assert(err, gc.IsNil)

	s.assertUnitsEqual(c, inst, got, checkVersion)
}

func (s *StoreBaseSuite) TestFindUnitCacheSupport(c *gc.C) {
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"

	// Create and persist a new Unit instance
	ns := s.st.ModelNamespace(modelUUID)
	inst := ns.NewUnit()
	populateFakeData(inst)
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Lookup persisted Unit twice using a new model namespace.
	// Then, check that the second lookup returned the same object instance
	// (pointer) as the first.
	ns = s.st.ModelNamespace(modelUUID)

	got1, err := ns.FindUnitByName(inst.GetName())
	c.Assert(err, gc.IsNil)

	got2, err := ns.FindUnitByName(inst.GetName())
	c.Assert(err, gc.IsNil)

	c.Assert(got1, gc.Equals, got2, gc.Commentf("expected both lookups to yield the same Unit pointer due to caching"))
}

func (s *StoreBaseSuite) TestUpdateUnit(c *gc.C) {
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"

	// Create and persist a new Unit instance
	ns := s.st.ModelNamespace(modelUUID)
	origInst := ns.NewUnit()
	populateFakeData(origInst)
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Lookup persisted Unit and randomize its fields (but retain the PK field)
	ns = s.st.ModelNamespace(modelUUID)
	gotInserted, err := ns.FindUnitByName(origInst.GetName())
	c.Assert(err, gc.IsNil)
	populateFakeData(gotInserted)
	gotInserted.SetName(origInst.GetName()) // retain original PK
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Now lookup the updated model and verify that the fields match
	ns = s.st.ModelNamespace(modelUUID)
	gotUpdated, err := ns.FindUnitByName(origInst.GetName())
	c.Assert(err, gc.IsNil)

	c.Assert(gotUpdated.ModelVersion(), gc.Equals, gotInserted.ModelVersion()+1, gc.Commentf("expected model version to be bumped after update"))
	s.assertUnitsEqual(c, gotInserted, gotUpdated, doNotCheckVersion)
}

func (s *StoreBaseSuite) TestAbortUnitUpdateIfVersionMismatch(c *gc.C) {
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"

	// Create and persist a new Unit instance
	ns := s.st.ModelNamespace(modelUUID)
	origInst := ns.NewUnit()
	populateFakeData(origInst)
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Create two separate namespaces that both try to mutate the same
	// model at the same time.
	nsA := s.st.ModelNamespace(modelUUID)
	instA, err := nsA.FindUnitByName(origInst.GetName())
	c.Assert(err, gc.IsNil)
	populateFakeData(instA)
	instA.SetName(origInst.GetName()) // retain original PK

	nsB := s.st.ModelNamespace(modelUUID)
	instB, err := nsB.FindUnitByName(origInst.GetName())
	c.Assert(err, gc.IsNil)
	populateFakeData(instB)
	instB.SetName(origInst.GetName()) // retain original PK

	// Commit instB changes first
	c.Assert(s.st.ApplyChanges(nsB), gc.IsNil)

	// Trying to commit instA changes should fail as the model version in
	// the store has been bumped after successfully committing the instB changes.
	err = s.st.ApplyChanges(nsA)
	c.Assert(err, gc.Equals, store.ErrStoreVersionMismatch, gc.Commentf("expected update to fail as the model version in the store has changed"))
}

func (s *StoreBaseSuite) TestDeleteUnit(c *gc.C) {
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"

	// Create and persist a new Unit instance
	ns := s.st.ModelNamespace(modelUUID)
	inst := ns.NewUnit()
	populateFakeData(inst)
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Check count
	count, err := ns.CountUnits()
	c.Assert(err, gc.IsNil)
	c.Assert(count, gc.Equals, 1, gc.Commentf("expected CountUnit() to return 1"))

	// Lookup persisted Unit using a new model namespace and delete it
	ns = s.st.ModelNamespace(modelUUID)
	c.Assert(err, gc.IsNil)

	got, err := ns.FindUnitByName(inst.GetName())
	c.Assert(err, gc.IsNil)

	got.Delete()
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Check count after deletion
	count, err = ns.CountUnits()
	c.Assert(err, gc.IsNil)
	c.Assert(count, gc.Equals, 0, gc.Commentf("expected CountUnit() to return 0"))
}

func (s *StoreBaseSuite) TestNamespaceSeparationForUnits(c *gc.C) {
	// Create and persist a new Unit instance scoped to modelUUID1
	modelUUID1 := "3a92d636-a8b8-11eb-bcbc-0242ac130002"
	ns1 := s.st.ModelNamespace(modelUUID1)
	inst1 := ns1.NewUnit()
	populateFakeData(inst1)
	c.Assert(s.st.ApplyChanges(ns1), gc.IsNil)

	// Create and persist a new Unit instance scoped to modelUUID2
	modelUUID2 := "99999999-a8b8-11eb-bcbc-0242ac130002"
	ns2 := s.st.ModelNamespace(modelUUID2)
	inst2 := ns2.NewUnit()
	populateFakeData(inst2)
	c.Assert(s.st.ApplyChanges(ns2), gc.IsNil)

	// Iterate all Unit instances scoped to modelUUID1 and ensure we
	// only get back inst1.
	ns1 = s.st.ModelNamespace(modelUUID1)
	instList := s.consumeUnitIterator(c, ns1.AllUnits())
	c.Assert(instList, gc.HasLen, 1, gc.Commentf("expected to get back only the models in NS1"))

	// Iterate all Unit instances scoped to modelUUID2 and ensure we
	// only get back inst2.
	ns2 = s.st.ModelNamespace(modelUUID2)
	instList = s.consumeUnitIterator(c, ns2.AllUnits())
	c.Assert(instList, gc.HasLen, 1, gc.Commentf("expected to get back only the models in NS2"))
}

func (s *StoreBaseSuite) TestFindUnitsByApplication(c *gc.C) {
	numInstances := 10

	// Create and persist a batch of Unit instances with the same application.
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"
	ns := s.st.ModelNamespace(modelUUID)

	var searchFieldTemplate string
	searchFieldVal := genScalarValue(reflect.TypeOf(searchFieldTemplate)).(string)

	for i := 0; i < numInstances; i++ {
		inst := ns.NewUnit()
		populateFakeData(inst)
		inst.SetApplication(searchFieldVal)
	}
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Use a new namespace to run the search query.
	ns = s.st.ModelNamespace(modelUUID)
	it := ns.FindUnitsByApplication(searchFieldVal)
	instList := s.consumeUnitIterator(c, it)
	c.Assert(instList, gc.HasLen, numInstances, gc.Commentf("expected FindUnitsByApplication() to return %d instances", numInstances))
}

func (s *StoreBaseSuite) TestUnitIteratorUsesCache(c *gc.C) {
	numInstances := 10

	// Create and persist a batch of Unit instances.
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"
	ns := s.st.ModelNamespace(modelUUID)
	for i := 0; i < numInstances; i++ {
		inst := ns.NewUnit()
		populateFakeData(inst)
	}
	c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

	// Use a new namespace to run the same search query twice. Due to the
	// use of the cache, both iterators must yield the same Unit
	// pointers.
	ns = s.st.ModelNamespace(modelUUID)
	instList1 := s.consumeUnitIterator(c, ns.AllUnits())
	instList2 := s.consumeUnitIterator(c, ns.AllUnits())
	// As the iterators may yield results in different order, sort by PK
	// before comparing the individual entries.
	sort.Slice(instList1, func(i, j int) bool { return instList1[i].GetName() < instList1[j].GetName() })
	sort.Slice(instList2, func(i, j int) bool { return instList2[i].GetName() < instList2[j].GetName() })
	c.Assert(len(instList1), gc.Equals, len(instList2))
	for i := 0; i < len(instList1); i++ {
		c.Assert(instList1[i], gc.Equals, instList2[i], gc.Commentf("expected both iterators to yield the same Unit pointers due to caching"))
	}
}

func (s *StoreBaseSuite) TestUnitIteratorLeakDetection(c *gc.C) {
	modelUUID := "3a92d636-a8b8-11eb-bcbc-0242ac130002"
	ns := s.st.ModelNamespace(modelUUID)

	// Get an iterator but call ApplyChanges without closing it first.
	it := ns.AllUnits()
	defer it.Close()
	err := s.st.ApplyChanges(ns)

	c.Assert(err, gc.ErrorMatches, "(?m).*unable to apply changes as the following iterators have not been closed.*")
}

func (s *StoreBaseSuite) assertUnitsEqual(c *gc.C, a, b *model.Unit, checkVersions bool) {
	c.Assert(a.GetName(), gc.DeepEquals, b.GetName(), gc.Commentf("value mismatch for field: name"))
	c.Assert(a.GetApplication(), gc.DeepEquals, b.GetApplication(), gc.Commentf("value mismatch for field: application"))
	c.Assert(a.GetSeries(), gc.DeepEquals, b.GetSeries(), gc.Commentf("value mismatch for field: series"))
	c.Assert(a.GetPrincipal(), gc.DeepEquals, b.GetPrincipal(), gc.Commentf("value mismatch for field: principal"))
	c.Assert(a.GetSubordinates(), gc.DeepEquals, b.GetSubordinates(), gc.Commentf("value mismatch for field: subordinates"))
	c.Assert(a.GetStorageAttachmentCount(), gc.DeepEquals, b.GetStorageAttachmentCount(), gc.Commentf("value mismatch for field: storageAttachmentCount"))
	c.Assert(a.GetMachineId(), gc.DeepEquals, b.GetMachineId(), gc.Commentf("value mismatch for field: machineId"))
	c.Assert(a.GetPasswordHash(), gc.DeepEquals, b.GetPasswordHash(), gc.Commentf("value mismatch for field: passwordHash"))

	if checkVersions {
		c.Assert(a.ModelVersion(), gc.Equals, b.ModelVersion(), gc.Commentf("version mismatch"))
	}
}

func (s *StoreBaseSuite) consumeUnitIterator(c *gc.C, it store.UnitIterator) []*model.Unit {
	defer func() {
		// Ensure iterator is always closed even if we get an error.
		it.Close()
	}()
	var res []*model.Unit
	for it.Next() {
		res = append(res, it.Unit())
	}
	c.Assert(it.Error(), gc.IsNil, gc.Commentf("encountered error while iterating Units"))
	c.Assert(it.Close(), gc.IsNil, gc.Commentf("encountered error while closing Unit iterator"))
	return res
}

func populateFakeData(in interface{}) interface{} {
	// If in is a pointer, dereference it first
	inputV := reflect.ValueOf(in)
	v := inputV
	if inputV.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		// Scan public fields and populate them with values
		structType := v.Type()
		for fIdx := 0; fIdx < v.NumField(); fIdx++ {
			structField := v.Field(fIdx)

			// Consult field name (in the struct def) and skip
			// private fields.
			fieldName := structType.Field(fIdx).Name
			if fieldName == "" || !unicode.IsUpper(rune(fieldName[0])) {
				continue
			}

			// Generate value template for the expected type and
			// pass it to populateFakeData. Then, populate the
			// field with the generated value.
			fieldIn := genSettableField(structField.Type())
			genVal := populateFakeData(fieldIn.Interface())

			// If the field is a non-ptr struct, genSettableField
			// created a pointer so setters can work; we need to
			// dereference the pointer before assigning.
			if structField.Type().Kind() == reflect.Struct {
				genVal = reflect.ValueOf(genVal).Elem().Interface()
			}
			structField.Set(reflect.ValueOf(genVal))
		}

		// Process Setters for auto-generated models; we are only interested
		// in methods operating on a struct pointer so we need to use
		// inputV (the struct pointer) instead of V.
		for mIdx := 0; mIdx < inputV.NumMethod(); mIdx++ {
			structMethod := inputV.Type().Method(mIdx)
			methodType := structMethod.Func.Type()
			// We only care for SetX functions operating on a pointer
			// of this struct (the ptr to the struct is the first arg).
			if !strings.HasPrefix(structMethod.Name, "Set") || methodType.NumIn() != 2 || methodType.In(0) != inputV.Type() {
				continue
			}

			// Generate a value template for the 2nd method arg and
			// pass it to populateFakeData. Then, invoke the method
			// with the generated value.
			fieldIn := genSettableField(methodType.In(1))
			genVal := populateFakeData(fieldIn.Interface())

			// If the field is a non-ptr struct, genSettableField
			// created a pointer so setters can work; we need to
			// dereference the pointer before assigning.
			if methodType.In(1).Kind() == reflect.Struct {
				genVal = reflect.ValueOf(genVal).Elem().Interface()
			}

			// Call method on struct pointer.
			_ = inputV.MethodByName(structMethod.Name).Call(
				[]reflect.Value{reflect.ValueOf(genVal).Convert(methodType.In(1))},
			)
		}
	case reflect.Slice:
		elemType := v.Type().Elem()
		for i := 0; i < rand.Intn(10)+1; i++ {
			fieldIn := genSettableField(elemType)
			genVal := populateFakeData(fieldIn.Interface())

			// If the field is a non-ptr struct, genSettableField
			// created a pointer so setters can work; we need to
			// dereference the pointer before assigning.
			if elemType.Kind() == reflect.Struct {
				genVal = reflect.ValueOf(genVal).Elem().Interface()
			}
			v = reflect.Append(v, reflect.ValueOf(genVal))
		}
		in = v.Interface()
	case reflect.Map:
		keyType := v.Type().Key()
		valType := v.Type().Elem()
		for i := 0; i < rand.Intn(10)+1; i++ {
			keyIn := genSettableField(keyType)
			genKey := populateFakeData(keyIn.Interface())

			valIn := genSettableField(valType)
			genVal := populateFakeData(valIn.Interface())

			// If the value field is a non-ptr struct,
			// genSettableField created a pointer so setters can
			// work; we need to dereference the pointer before
			// assigning.
			if valType.Kind() == reflect.Struct {
				genVal = reflect.ValueOf(genVal).Elem().Interface()
			}

			v.SetMapIndex(reflect.ValueOf(genKey), reflect.ValueOf(genVal))
		}
	default:
		return genScalarValue(v.Type())
	}

	return in
}

func genSettableField(typ reflect.Type) reflect.Value {
	switch typ.Kind() {
	case reflect.Struct:
		// If the target is a non-pointer struct field,
		// pass a pointer to a new struct instance so
		// that its fields can be set via reflection
		// and dereference the value before assigning
		// to this field.
		return reflect.New(typ)
	case reflect.Ptr:
		// If it's a pointer, allocate an instance of the
		// underlying type.
		return reflect.New(typ.Elem())
	case reflect.Map:
		return reflect.MakeMap(typ)
	default:
		return reflect.Zero(typ)
	}
}

func genScalarValue(typ reflect.Type) interface{} {
	alphabet := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	switch typ.Kind() {
	case reflect.String:
		res := make([]byte, 10)
		for i := 0; i < len(res); i++ {
			res[i] = alphabet[rand.Intn(len(alphabet))]
		}
		return string(res)
	case reflect.Bool:
		if rand.Int()%2 == 0 {
			return false
		}
		return true
	case reflect.Int8:
		return int8(rand.Intn(math.MaxInt8))
	case reflect.Uint8:
		return uint8(rand.Intn(math.MaxUint8))
	case reflect.Int16:
		return int16(rand.Intn(math.MaxInt16))
	case reflect.Uint16:
		return uint16(rand.Intn(math.MaxUint16))
	case reflect.Int32:
		return int32(rand.Intn(math.MaxInt32))
	case reflect.Uint32:
		return uint32(rand.Intn(math.MaxUint32))
	case reflect.Int64:
		return int64(rand.Intn(math.MaxInt64))
	case reflect.Int:
		return rand.Intn(math.MaxInt64)
	case reflect.Uint64:
		return uint64(rand.Int())
	case reflect.Uint:
		return uint(rand.Int())
	case reflect.Float32:
		return rand.Float32()
	case reflect.Float64:
		return rand.Float64()
	default:
		panic(fmt.Errorf("genScalarValue: unable to generate scalar value of type %s", typ))
		return nil
	}
}
