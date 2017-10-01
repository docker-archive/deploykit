package instance

import (
	"testing"

	testutil "github.com/docker/infrakit/pkg/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestInfrakitRackhd(t *testing.T) {
	if testutil.SkipTests("rackhd") {
		t.SkipNow()
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "Infrakit.Rackhd.Plugin.Instance Suite")
}
