package test

import (
	"errors"
	"path"
	goruntime "runtime"

	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(eksv1.AddToScheme(scheme))
}

func StartEnvTest(testEnv *envtest.Environment) (*rest.Config, runtimeclient.Client, error) {
	// Get the root of the current file to use in CRD paths.
	_, filename, _, _ := goruntime.Caller(0) //nolint:dogsled
	root := path.Join(path.Dir(filename), "..", "..", "..", "eks-operator")

	testEnv.CRDs = []*apiextensionsv1.CustomResourceDefinition{
		// Add later if needed.
	}
	testEnv.CRDDirectoryPaths = []string{
		path.Join(root, "charts", "eks-operator-crd", "templates"),
	}
	testEnv.ErrorIfCRDPathMissing = true

	cfg, err := testEnv.Start()
	if err != nil {
		return nil, nil, err
	}

	if cfg == nil {
		return nil, nil, errors.New("envtest.Environment.Start() returned nil config")
	}

	cl, err := runtimeclient.New(cfg, runtimeclient.Options{Scheme: scheme})
	if err != nil {
		return nil, nil, err
	}

	return cfg, cl, nil
}

func StopEnvTest(testEnv *envtest.Environment) error {
	return testEnv.Stop()
}
