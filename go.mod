module github.com/rancher/eks-operator

go 1.21

replace k8s.io/client-go => k8s.io/client-go v0.28.6

require (
	github.com/aws/aws-sdk-go v1.50.38
	github.com/aws/aws-sdk-go-v2 v1.26.1
	github.com/aws/aws-sdk-go-v2/config v1.27.5
	github.com/aws/aws-sdk-go-v2/credentials v1.17.7
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.50.0
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.157.0
	github.com/aws/aws-sdk-go-v2/service/eks v1.42.1
	github.com/aws/aws-sdk-go-v2/service/iam v1.32.0
	github.com/blang/semver v3.5.1+incompatible
	github.com/drone/envsubst/v2 v2.0.0-20210730161058-179042472c46
	github.com/golang/mock v1.6.0
	github.com/onsi/ginkgo/v2 v2.17.1
	github.com/onsi/gomega v1.32.0
	github.com/pkg/errors v0.9.1
	github.com/rancher-sandbox/ele-testhelpers v0.0.0-20221213084338-a8ffdd2b87e3
	github.com/rancher/lasso v0.0.0-20240123150939-7055397d6dfa
	github.com/rancher/rancher/pkg/apis v0.0.0-20240104144633-360c02b3761f
	github.com/rancher/wrangler-api v0.6.1-0.20200427172631-a7c2f09b783e
	github.com/rancher/wrangler/v2 v2.1.3
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.9.0
	golang.org/x/net v0.24.0
	k8s.io/api v0.28.8
	k8s.io/apiextensions-apiserver v0.28.8
	k8s.io/apimachinery v0.28.8
	k8s.io/apiserver v0.28.8
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.15.3
	sigs.k8s.io/yaml v1.4.0
)

require (
	github.com/blang/semver/v4 v4.0.0 // indirect
	k8s.io/component-base v0.28.8 // indirect
)

require (
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.15.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.5 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.5 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.11.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.20.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.23.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.28.4 // indirect
	github.com/aws/smithy-go v1.20.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.10.2 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/zapr v1.2.4 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20210720184732-4bb14d4b1be1 // indirect
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
	github.com/rancher/aks-operator v1.3.0-rc1 // indirect
	github.com/rancher/fleet/pkg/apis v0.0.0-20231017140638-93432f288e79 // indirect
	github.com/rancher/gke-operator v1.3.0-rc2 // indirect
	github.com/rancher/norman v0.0.0-20240207153100-3bb70b772b52 // indirect
	github.com/rancher/rke v1.5.0 // indirect
	github.com/rancher/wrangler v1.1.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/oauth2 v0.15.0 // indirect
	golang.org/x/sync v0.6.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/term v0.19.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.17.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/code-generator v0.28.8 // indirect
	k8s.io/gengo v0.0.0-20230306165830-ab3349d207d4 // indirect
	k8s.io/klog/v2 v2.100.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230717233707-2695361300d9 // indirect
	k8s.io/kubernetes v1.27.9 // indirect
	k8s.io/utils v0.0.0-20230505201702-9f6742963106 // indirect
	sigs.k8s.io/cli-utils v0.28.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
)
