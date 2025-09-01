module github.com/rancher/eks-operator

go 1.23.0

replace k8s.io/client-go => k8s.io/client-go v0.28.9

require (
	github.com/aws/aws-sdk-go v1.49.24
	github.com/blang/semver v3.5.1+incompatible
	github.com/drone/envsubst/v2 v2.0.0-20210730161058-179042472c46
	github.com/golang/mock v1.6.0
	github.com/onsi/ginkgo/v2 v2.25.1
	github.com/onsi/gomega v1.38.2
	github.com/pkg/errors v0.9.1
	github.com/rancher-sandbox/ele-testhelpers v0.0.0-20221213084338-a8ffdd2b87e3
	github.com/rancher/lasso v0.0.0-20240123150939-7055397d6dfa
	github.com/rancher/rancher/pkg/apis v0.0.0-20240613212755-3021cf92ff9f
	github.com/rancher/wrangler-api v0.6.1-0.20200427172631-a7c2f09b783e
	github.com/rancher/wrangler/v2 v2.1.4
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.10.0
	k8s.io/api v0.28.9
	k8s.io/apiextensions-apiserver v0.28.9
	k8s.io/apimachinery v0.28.9
	k8s.io/apiserver v0.28.9
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.16.6
	sigs.k8s.io/yaml v1.5.0
)

require (
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/zapr v1.2.4 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20250403155104-27863c87afa6 // indirect
	github.com/google/uuid v1.5.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.16.0 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/common v0.44.0 // indirect
	github.com/prometheus/procfs v0.10.1 // indirect
	github.com/rancher/aks-operator v1.2.2 // indirect
	github.com/rancher/fleet/pkg/apis v0.9.1-rc.2.0.20240213164401-2c6b1019687c // indirect
	github.com/rancher/gke-operator v1.2.2 // indirect
	github.com/rancher/norman v0.0.0-20240206180703-6eda4bc94b4c // indirect
	github.com/rancher/rke v1.5.11-rc1 // indirect
	github.com/rancher/wrangler v1.1.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.25.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.27.0 // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/oauth2 v0.20.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/term v0.34.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.36.0 // indirect
	google.golang.org/protobuf v1.36.7 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/code-generator v0.28.9 // indirect
	k8s.io/component-base v0.28.9 // indirect
	k8s.io/gengo v0.0.0-20230306165830-ab3349d207d4 // indirect
	k8s.io/klog/v2 v2.100.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230717233707-2695361300d9 // indirect
	k8s.io/kubernetes v1.28.9 // indirect
	k8s.io/utils v0.0.0-20230505201702-9f6742963106 // indirect
	sigs.k8s.io/cli-utils v0.28.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
)
