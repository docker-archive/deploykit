package libmachete

import (
	"github.com/docker/libmachete"
	. "gopkg.in/check.v1"
	"testing"
)

func TestApi(t *testing.T) { TestingT(t) }

type TestSuiteApi struct {
}

var _ = Suite(&TestSuiteApi{})

func (suite *TestSuiteApi) SetUpSuite(c *C) {
}

func (suite *TestSuiteApi) TearDownSuite(c *C) {
}
