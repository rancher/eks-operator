package main

import (
	"fmt"
	"os"

	v12 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	_ "github.com/rancher/wrangler-api/pkg/generated/controllers/apiextensions.k8s.io"
	controllergen "github.com/rancher/wrangler/pkg/controller-gen"
	"github.com/rancher/wrangler/pkg/controller-gen/args"
	"github.com/rancher/wrangler/pkg/crd"
	"github.com/rancher/wrangler/pkg/yaml"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func main() {
	controllergen.Run(args.Options{
		OutputPackage: "github.com/rancher/eks-operator/pkg/generated",
		Boilerplate:   "pkg/codegen/boilerplate.go.txt",
		Groups: map[string]args.Group{
			"eks.cattle.io": {
				Types: []interface{}{
					v12.EKSClusterConfig{},
				},
				GenerateTypes: true,
			},
			// Optionally you can use wrangler-api project which
			// has a lot of common kubernetes APIs already generated.
			// In this controller we will use wrangler-api for apps api group
			"": {
				Types: []interface{}{
					v1.Pod{},
					v1.Node{},
					v1.Secret{},
				},
				InformersPackage: "k8s.io/client-go/informers",
				ClientSetPackage: "k8s.io/client-go/kubernetes",
				ListersPackage:   "k8s.io/client-go/listers",
			},
		},
	})

	eksClusterConfig := newCRD(&v12.EKSClusterConfig{}, func(c crd.CRD) crd.CRD {
		c.ShortNames = []string{"ekscc"}
		return c
	})

	obj, err := eksClusterConfig.ToCustomResourceDefinition()
	if err != nil {
		panic(err)
	}

	eksCCYaml, err := yaml.Export(&obj)
	if err != nil {
		panic(err)
	}

	if err := saveCRDYaml("eksclusterconfig", string(eksCCYaml)); err != nil {
		panic(err)
	}

	fmt.Printf("obj yaml: %s", eksCCYaml)
}

func newCRD(obj interface{}, customize func(crd.CRD) crd.CRD) crd.CRD {
	crd := crd.CRD{
		GVK: schema.GroupVersionKind{
			Group:   "eks.cattle.io",
			Version: "v1",
		},
		Status:       true,
		SchemaObject: obj,
	}
	if customize != nil {
		crd = customize(crd)
	}
	return crd
}

func saveCRDYaml(name, yaml string) error {
	filename := fmt.Sprintf("./crds/%s.yaml", name)
	save, err := os.Create(filename)
	if err != nil {
		return err
	}

	defer save.Close()
	if err := save.Chmod(0755); err != nil {
		return err
	}

	if _, err := fmt.Fprint(save, yaml); err != nil {
		return err
	}

	return nil
}
