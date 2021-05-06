package {{basePkgName .TargetPkg}}
{{$modelPkgBase := basePkgName .ModelPkg}}

//
// The contents of this file have been auto-generated. Do not modify.
//

import (
	gc "gopkg.in/check.v1"

	"{{.StoreTestPkg}}"
)

var _ = gc.Suite(&MongoClientTxnSuite{})
var _ = gc.Suite(&MongoServerTxnSuite{})

type MongoBaseSuite struct {
	{{basePkgName .StoreTestPkg}}.StoreBaseSuite
	st *Mongo
}

func (s *MongoBaseSuite) SetUpTest(c *gc.C) {
	s.StoreBaseSuite.SetUpTest(c)
}

func (s *MongoBaseSuite) resetDB(c *gc.C) {
	c.Assert(s.st.Close(), gc.IsNil)
	c.Assert(s.st.Open(), gc.IsNil)
	c.Assert(s.st.mgoDB.DropDatabase(), gc.IsNil)

	// Close and reopen to use a new DB and re-establish any indices.
	c.Assert(s.st.Close(), gc.IsNil)
	c.Assert(s.st.Open(), gc.IsNil)
}

type MongoClientTxnSuite struct {
	MongoBaseSuite
}

func (s *MongoClientTxnSuite) SetUpSuite(c *gc.C) {
	s.st = NewMongo("localhost", "test", false) // use client-side txns.
	s.StoreBaseSuite.Configure(s.st, s.resetDB)
}

func (s *MongoClientTxnSuite) SetUpTest(c *gc.C) {
	s.MongoBaseSuite.SetUpTest(c)
}

type MongoServerTxnSuite struct {
	MongoBaseSuite
}

func (s *MongoServerTxnSuite) SetUpSuite(c *gc.C) {
	s.st = NewMongo("localhost", "test", true) // use server-side txns.
	s.StoreBaseSuite.Configure(s.st, s.resetDB)
}

func (s *MongoServerTxnSuite) SetUpTest(c *gc.C) {
	s.MongoBaseSuite.SetUpTest(c)
}
