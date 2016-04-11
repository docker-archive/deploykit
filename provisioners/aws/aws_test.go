package aws

import (
	api "github.com/docker/libmachete"
	. "gopkg.in/check.v1"
	"testing"
)

func TestAws(t *testing.T) { TestingT(t) }

type TestSuiteAws struct {
}

var _ = Suite(&TestSuiteAws{})

func (suite *TestSuiteAws) SetUpSuite(c *C) {
}

func (suite *TestSuiteAws) TearDownSuite(c *C) {
}

func (suite *TestSuiteAws) TestGetProvisioner(c *C) {
	aws := api.GetProvisioner("aws")
	c.Assert(aws, Not(IsNil))
}
