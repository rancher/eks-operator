//go:generate go run pkg/codegen/cleanup/main.go
//go:generate go run pkg/codegen/main.go

package main

import (
	"flag"

	"github.com/rancher/eks-operator/controller"
	eksv1 "github.com/rancher/eks-operator/pkg/generated/controllers/eks.cattle.io"
	"github.com/rancher/wrangler/v2/pkg/generated/controllers/apps"
	core3 "github.com/rancher/wrangler/v2/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/v2/pkg/kubeconfig"
	"github.com/rancher/wrangler/v2/pkg/signals"
	"github.com/rancher/wrangler/v2/pkg/start"
	"github.com/sirupsen/logrus"
)

var (
	masterURL      string
	kubeconfigFile string
)

func init() {
	flag.StringVar(&kubeconfigFile, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.Parse()
}

func main() {
	// set up signals so we handle the first shutdown signal gracefully
	ctx := signals.SetupSignalContext()

	// This will load the kubeconfig file in a style the same as kubectl
	cfg, err := kubeconfig.GetNonInteractiveClientConfig(kubeconfigFile).ClientConfig()
	if err != nil {
		logrus.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	// Generated apps controller
	apps := apps.NewFactoryFromConfigOrDie(cfg)
	// core
	core, err := core3.NewFactoryFromConfig(cfg)
	if err != nil {
		logrus.Fatalf("Error building core factory: %s", err.Error())
	}

	// Generated sample controller
	eks, err := eksv1.NewFactoryFromConfig(cfg)
	if err != nil {
		logrus.Fatalf("Error building eks factory: %s", err.Error())
	}

	// The typical pattern is to build all your controller/clients then just pass to each handler
	// the bare minimum of what they need.  This will eventually help with writing tests.  So
	// don't pass in something like kubeClient, apps, or sample
	controller.Register(ctx,
		core.Core().V1().Secret(),
		eks.Eks().V1().EKSClusterConfig())

	// Start all the controllers
	if err := start.All(ctx, 3, apps, eks, core); err != nil {
		logrus.Fatalf("Error starting: %s", err.Error())
	}

	<-ctx.Done()
}
