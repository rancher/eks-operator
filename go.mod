module github.com/rancher/eks-operator

go 1.13

replace k8s.io/client-go => k8s.io/client-go v0.18.0

require (
	github.com/aws/aws-sdk-go v1.30.22
	github.com/blang/semver v3.5.0+incompatible
	github.com/rancher/lasso v0.0.0-20200820172840-0e4cc0ef5cb0
	github.com/rancher/wrangler v0.6.2-0.20200822010948-6d667521af49
	github.com/rancher/wrangler-api v0.6.1-0.20200427172631-a7c2f09b783e
	github.com/sirupsen/logrus v1.4.2
	k8s.io/api v0.18.8
	k8s.io/apiextensions-apiserver v0.18.0
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
)
