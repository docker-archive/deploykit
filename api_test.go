package libmachete

import (
	. "gopkg.in/check.v1"
	"testing"
)

func TestAPI(t *testing.T) { TestingT(t) }

type TestSuiteAPI struct {
}

var _ = Suite(&TestSuiteAPI{})

func (suite *TestSuiteAPI) SetUpSuite(c *C) {
}

func (suite *TestSuiteAPI) TearDownSuite(c *C) {
}
