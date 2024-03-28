package eks

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
)

var (
	ctx = context.Background()
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EKS services Suite")
}
