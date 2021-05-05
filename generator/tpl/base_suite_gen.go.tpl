package {{basePkgName .TargetPkg}}

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

	"{{.ModelPkg}}"
	"{{.StorePkg}}"
)

{{$modelPkg := basePkgName .ModelPkg}}
{{$storePkg := basePkgName .StorePkg}}

const (
	checkVersion = true
	doNotCheckVersion = false
)

// StoreBaseSuite implements a store-agnostic test-suite harness which allows
// testing the behavior of model-store interactions with any supported store
// implementations.
type StoreBaseSuite struct {
	// A Store instance which the embedding test suite provides via
	// ConfigureSuite().
	st {{$storePkg}}.Store

	// A cleanup function to truncate the store contents which the embedding
	// test suite provides via ConfigureSuite().
	cleanupFunc func(*gc.C)
}

// Configure the suite parameters. This method must be called before running
// any of the test suite methods or an error will be raised.
func (s *StoreBaseSuite) Configure(st {{$storePkg}}.Store, cleanupFunc func(*gc.C)) {
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

{{with nonEmbeddedModels .Models}}
{{range .}}
	{{- $model := . -}}
	{{- $modelName := .Name.Public -}}
	func (s *StoreBaseSuite) TestInsert{{$modelName}}(c *gc.C){
		modelUUID :="3a92d636-a8b8-11eb-bcbc-0242ac130002"

		// Create and persist a new {{$modelName}} instance
		ns := s.st.ModelNamespace(modelUUID)
		c.Assert(ns.ModelUUID(), gc.Equals, modelUUID, gc.Commentf("the ModelNamespace implementation does not correctly track the associated modelUUID"))
		inst := ns.New{{$modelName}}()
		populateFakeData(inst)
		c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

		// Lookup persisted {{$modelName}} using a new model namespace
		ns = s.st.ModelNamespace(modelUUID)
		{{range $model.Fields}}
		{{- if .Flags.IsPK}}
			got, err := ns.Find{{$modelName}}By{{.Name.Public}}(inst.{{getter .}}())
		{{- end -}}
		{{end}}
		c.Assert(err, gc.IsNil)

		s.assert{{$modelName}}sEqual(c, inst, got, checkVersion)
	}

	{{if $model.HasNullableFields}}
	func (s *StoreBaseSuite) TestInsert{{$modelName}}WithNilFields(c *gc.C){
		modelUUID :="3a92d636-a8b8-11eb-bcbc-0242ac130002"

		// Create and persist a new {{$modelName}} instance
		ns := s.st.ModelNamespace(modelUUID)
		c.Assert(ns.ModelUUID(), gc.Equals, modelUUID, gc.Commentf("the ModelNamespace implementation does not correctly track the associated modelUUID"))
		inst := ns.New{{$modelName}}()
		populateFakeData(inst)

		// Clear nullable fields
		inst{{- range $model.Fields -}}
		{{- if .Flags.IsNullable -}}.
			Set{{.Name.Public}}(nil)
		{{- end -}}
		{{- end}}

		c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

		// Lookup persisted {{$modelName}} using a new model namespace
		ns = s.st.ModelNamespace(modelUUID)
		{{range $model.Fields}}
		{{- if .Flags.IsPK}}
			got, err := ns.Find{{$modelName}}By{{.Name.Public}}(inst.{{getter .}}())
		{{- end -}}
		{{end}}
		c.Assert(err, gc.IsNil)

		s.assert{{$modelName}}sEqual(c, inst, got, checkVersion)
	}
	{{end}}

	func (s *StoreBaseSuite) TestFind{{$modelName}}CacheSupport(c *gc.C){
		modelUUID :="3a92d636-a8b8-11eb-bcbc-0242ac130002"

		// Create and persist a new {{$modelName}} instance
		ns := s.st.ModelNamespace(modelUUID)
		inst := ns.New{{$modelName}}()
		populateFakeData(inst)
		c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

		// Lookup persisted {{$modelName}} twice using a new model namespace.
		// Then, check that the second lookup returned the same object instance
		// (pointer) as the first.
		ns = s.st.ModelNamespace(modelUUID)
		{{range $model.Fields}}
		{{- if .Flags.IsPK}}
			got1, err := ns.Find{{$modelName}}By{{.Name.Public}}(inst.{{getter .}}())
			c.Assert(err, gc.IsNil)

			got2, err := ns.Find{{$modelName}}By{{.Name.Public}}(inst.{{getter .}}())
			c.Assert(err, gc.IsNil)

			c.Assert(got1, gc.Equals, got2, gc.Commentf("expected both lookups to yield the same {{$modelName}} pointer due to caching"))
		{{- end -}}
		{{end}}
	}

	func (s *StoreBaseSuite) TestUpdate{{$modelName}}(c *gc.C){
		modelUUID :="3a92d636-a8b8-11eb-bcbc-0242ac130002"

		// Create and persist a new {{$modelName}} instance
		ns := s.st.ModelNamespace(modelUUID)
		origInst := ns.New{{$modelName}}()
		populateFakeData(origInst)
		c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

		{{range $model.Fields}}
		{{- if .Flags.IsPK}}
			// Lookup persisted {{$modelName}} and randomize its fields (but retain the PK field)
			ns = s.st.ModelNamespace(modelUUID)
			gotInserted, err := ns.Find{{$modelName}}By{{.Name.Public}}( origInst.{{getter .}}())
			c.Assert(err, gc.IsNil)
			populateFakeData(gotInserted)
			gotInserted.Set{{.Name.Public}}(origInst.{{getter .}}()) // retain original PK
			c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

			// Now lookup the updated model and verify that the fields match
			ns = s.st.ModelNamespace(modelUUID)
			gotUpdated, err := ns.Find{{$modelName}}By{{.Name.Public}}( origInst.{{getter .}}())
			c.Assert(err, gc.IsNil)
		{{- end -}}
		{{end}}

		c.Assert(gotUpdated.ModelVersion(), gc.Equals, gotInserted.ModelVersion() + 1, gc.Commentf("expected model version to be bumped after update"))
		s.assert{{$modelName}}sEqual(c, gotInserted, gotUpdated, doNotCheckVersion)
	}

	func (s *StoreBaseSuite) TestAbort{{$modelName}}UpdateIfVersionMismatch(c *gc.C){
		modelUUID :="3a92d636-a8b8-11eb-bcbc-0242ac130002"

		// Create and persist a new {{$modelName}} instance
		ns := s.st.ModelNamespace(modelUUID)
		origInst := ns.New{{$modelName}}()
		populateFakeData(origInst)
		c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

		{{range $model.Fields}}
		{{- if .Flags.IsPK}}
			// Create two separate namespaces that both try to mutate the same
			// model at the same time.
			nsA := s.st.ModelNamespace(modelUUID)
			instA, err := nsA.Find{{$modelName}}By{{.Name.Public}}( origInst.{{getter .}}())
			c.Assert(err, gc.IsNil)
			populateFakeData(instA)
			instA.Set{{.Name.Public}}(origInst.{{getter .}}()) // retain original PK

			nsB := s.st.ModelNamespace(modelUUID)
			instB, err := nsB.Find{{$modelName}}By{{.Name.Public}}( origInst.{{getter .}}())
			c.Assert(err, gc.IsNil)
			populateFakeData(instB)
			instB.Set{{.Name.Public}}(origInst.{{getter .}}()) // retain original PK

			// Commit instB changes first
			c.Assert(s.st.ApplyChanges(nsB), gc.IsNil)

			// Trying to commit instA changes should fail as the model version in 
			// the store has been bumped after successfully committing the instB changes.
			err = s.st.ApplyChanges(nsA)
			c.Assert(err, gc.Equals, {{$storePkg}}.ErrStoreVersionMismatch, gc.Commentf("expected update to fail as the model version in the store has changed"))
		{{- end -}}
		{{end}}
	}

	func (s *StoreBaseSuite) TestDelete{{$modelName}}(c *gc.C){
		modelUUID :="3a92d636-a8b8-11eb-bcbc-0242ac130002"

		// Create and persist a new {{$modelName}} instance
		ns := s.st.ModelNamespace(modelUUID)
		inst := ns.New{{$modelName}}()
		populateFakeData(inst)
		c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

		// Check count
		count, err := ns.Count{{$modelName}}s()
		c.Assert(err, gc.IsNil)
		c.Assert(count, gc.Equals, 1, gc.Commentf("expected Count{{$modelName}}() to return 1"))

		// Lookup persisted {{$modelName}} using a new model namespace and delete it
		ns = s.st.ModelNamespace(modelUUID)
		c.Assert(err, gc.IsNil)
		{{range $model.Fields}}
		{{- if .Flags.IsPK}}
			got, err := ns.Find{{$modelName}}By{{.Name.Public}}(inst.{{getter .}}())
		{{- end -}}
		{{end}}
		c.Assert(err, gc.IsNil)

		got.Delete()
		c.Assert(s.st.ApplyChanges(ns), gc.IsNil)

		// Check count after deletion
		count, err = ns.Count{{$modelName}}s()
		c.Assert(err, gc.IsNil)
		c.Assert(count, gc.Equals, 0, gc.Commentf("expected Count{{$modelName}}() to return 0"))
	}

	func (s *StoreBaseSuite) TestNamespaceSeparationFor{{$modelName}}s(c *gc.C){
		// Create and persist a new {{$modelName}} instance scoped to modelUUID1
		modelUUID1 :="3a92d636-a8b8-11eb-bcbc-0242ac130002"
		ns1 := s.st.ModelNamespace(modelUUID1)
		inst1 := ns1.New{{$modelName}}()
		populateFakeData(inst1)
		c.Assert(s.st.ApplyChanges(ns1), gc.IsNil)

		// Create and persist a new {{$modelName}} instance scoped to modelUUID2
		modelUUID2 :="99999999-a8b8-11eb-bcbc-0242ac130002"
		ns2 := s.st.ModelNamespace(modelUUID2)
		inst2 := ns2.New{{$modelName}}()
		populateFakeData(inst2)
		c.Assert(s.st.ApplyChanges(ns2), gc.IsNil)

		// Iterate all {{$modelName}} instances scoped to modelUUID1 and ensure we
		// only get back inst1.
		ns1 = s.st.ModelNamespace(modelUUID1)
		instList := s.consume{{$modelName}}Iterator(c, ns1.All{{$modelName}}s())
		c.Assert(instList, gc.HasLen, 1, gc.Commentf("expected to get back only the models in NS1"))

		// Iterate all {{$modelName}} instances scoped to modelUUID2 and ensure we
		// only get back inst2.
		ns2 = s.st.ModelNamespace(modelUUID2)
		instList = s.consume{{$modelName}}Iterator(c, ns2.All{{$modelName}}s())
		c.Assert(instList, gc.HasLen, 1, gc.Commentf("expected to get back only the models in NS2"))
	}

	{{range $field := $model.Fields}}
	{{if .Flags.IsFindBy}}
	func (s *StoreBaseSuite) TestFind{{$modelName}}sBy{{$field.Name.Public}}(c *gc.C){
		numInstances := 10

		// Create and persist a batch of {{$modelName}} instances with the same {{.Name.Private}}.
		modelUUID :="3a92d636-a8b8-11eb-bcbc-0242ac130002"
		ns := s.st.ModelNamespace(modelUUID)

		var searchFieldTemplate {{$field.Type.Qualified}}
		searchFieldVal := genScalarValue(reflect.TypeOf(searchFieldTemplate)).({{$field.Type.Qualified}})

		for i:=0;i<numInstances;i++{
			inst := ns.New{{$modelName}}()
			populateFakeData(inst)
			inst.Set{{.Name.Public}}(searchFieldVal)
		}
		c.Assert(s.st.ApplyChanges(ns), gc.IsNil)
		
		// Use a new namespace to run the search query.
		ns = s.st.ModelNamespace(modelUUID)
		it := ns.Find{{$modelName}}sBy{{$field.Name.Public}}(searchFieldVal)
		instList := s.consume{{$modelName}}Iterator(c, it)
		c.Assert(instList, gc.HasLen, numInstances, gc.Commentf("expected Find{{$modelName}}sBy{{$field.Name.Public}}() to return %d instances", numInstances))
	}
	{{- end -}}
	{{- end}}

	func (s *StoreBaseSuite) Test{{$modelName}}IteratorUsesCache(c *gc.C){
		numInstances := 10

		// Create and persist a batch of {{$modelName}} instances.
		modelUUID :="3a92d636-a8b8-11eb-bcbc-0242ac130002"
		ns := s.st.ModelNamespace(modelUUID)
		for i:=0;i<numInstances;i++{
			inst := ns.New{{$modelName}}()
			populateFakeData(inst)
		}
		c.Assert(s.st.ApplyChanges(ns), gc.IsNil)
		
		// Use a new namespace to run the same search query twice. Due to the
		// use of the cache, both iterators must yield the same {{$modelName}}
		// pointers.
		ns = s.st.ModelNamespace(modelUUID)
		instList1 := s.consume{{$modelName}}Iterator(c, ns.All{{$modelName}}s())
		instList2 := s.consume{{$modelName}}Iterator(c, ns.All{{$modelName}}s())

		{{- range .Fields}}{{if .Flags.IsPK}}
			// As the iterators may yield results in different order, sort by PK
			// before comparing the individual entries.
			sort.Slice(instList1, func(i, j int) bool { return instList1[i].{{getter .}}() < instList1[j].{{getter .}}() })
			sort.Slice(instList2, func(i, j int) bool { return instList2[i].{{getter .}}() < instList2[j].{{getter .}}() })
		{{end -}}
		{{end -}}

		c.Assert(len(instList1), gc.Equals, len(instList2))
		for i:=0;i<len(instList1);i++{
			c.Assert(instList1[i], gc.Equals, instList2[i], gc.Commentf("expected both iterators to yield the same {{$modelName}} pointers due to caching"))
		}
	}
	
	func (s *StoreBaseSuite) Test{{$modelName}}IteratorLeakDetection(c *gc.C){
		modelUUID :="3a92d636-a8b8-11eb-bcbc-0242ac130002"
		ns := s.st.ModelNamespace(modelUUID)

		// Get an iterator but call ApplyChanges without closing it first.
		it := ns.All{{$modelName}}s()
		defer it.Close()
		err := s.st.ApplyChanges(ns)

		c.Assert(err, gc.ErrorMatches, "(?m).*unable to apply changes as the following iterators have not been closed.*")
	}

	func (s *StoreBaseSuite) assert{{$modelName}}sEqual(c *gc.C, a, b *{{$modelPkg}}.{{$modelName}}, checkVersions bool) {
		{{- range .Fields}}
			c.Assert(a.{{getter .}}(), gc.DeepEquals, b.{{getter .}}(), gc.Commentf("value mismatch for field: {{.Name.Private}}"))
		{{- end}}

		if checkVersions {
			c.Assert(a.ModelVersion(), gc.Equals, b.ModelVersion(), gc.Commentf("version mismatch"))
		}
	}

	func (s *StoreBaseSuite) consume{{$modelName}}Iterator(c *gc.C, it {{$storePkg}}.{{$modelName}}Iterator) []*{{$modelPkg}}.{{$modelName}} {
		defer func(){
			// Ensure iterator is always closed even if we get an error.
			it.Close()
		}()
		var res []*{{$modelPkg}}.{{$modelName}}
		for it.Next() {
			res = append(res, it.{{$modelName}}())	
		}
		c.Assert(it.Error(), gc.IsNil, gc.Commentf("encountered error while iterating {{$modelName}}s"))
		c.Assert(it.Close(), gc.IsNil, gc.Commentf("encountered error while closing {{$modelName}} iterator"))
		return res
	}
{{end}}
{{end}}

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
