package instance

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestInfrakitRackhd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Infrakit.Rackhd.Plugin.Instance Suite")
}
