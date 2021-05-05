package {{basePkgName .TargetPkg}}
{{$modelPkgBase := basePkgName .ModelPkg}}

//
// The contents of this file have been auto-generated. Do not modify.
//

import (
	gc "gopkg.in/check.v1"

	"{{.StoreTestPkg}}"
)

var _ = gc.Suite(&InMemorySuite{})

type InMemorySuite struct {
	{{basePkgName .StoreTestPkg}}.StoreBaseSuite
	st *InMemory
}

func (s *InMemorySuite) SetUpSuite(c *gc.C) {
	s.st = NewInMemory()
	s.StoreBaseSuite.Configure(s.st, s.resetDB)
}

func (s *InMemorySuite) SetUpTest(c *gc.C) {
	s.StoreBaseSuite.SetUpTest(c)
}

func (s *InMemorySuite) resetDB(c *gc.C) {
	c.Assert(s.st.Close(), gc.IsNil)
	c.Assert(s.st.Open(), gc.IsNil)
}
