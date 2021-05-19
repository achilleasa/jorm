package backend

//
// The contents of this file have been auto-generated. Do not modify.
//

import (
	"os"
	"path/filepath"

	gc "gopkg.in/check.v1"

	"github.com/achilleasa/jorm/store/storetest"
)

var _ = gc.Suite(&SQLiteSuite{})

type SQLiteSuite struct {
	storetest.StoreBaseSuite
	st *SQLite
}

func (s *SQLiteSuite) SetUpSuite(c *gc.C) {
	dbDir := c.MkDir()
	s.st = NewSQLite(filepath.Join(dbDir, "db.sqlite"))
	s.StoreBaseSuite.Configure(s.st, s.resetDB)
}

func (s *SQLiteSuite) SetUpTest(c *gc.C) {
	s.StoreBaseSuite.SetUpTest(c)
}

func (s *SQLiteSuite) resetDB(c *gc.C) {
	c.Assert(s.st.Close(), gc.IsNil)
	if _, err := os.Stat(s.st.dbFile); err == nil {
		c.Assert(os.Remove(s.st.dbFile), gc.IsNil)
	}
	c.Assert(s.st.Open(), gc.IsNil)
}
