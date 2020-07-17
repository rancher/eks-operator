module github.com/rancher/eks-operator

go 1.13

replace k8s.io/client-go => k8s.io/client-go v0.18.0

require (
	github.com/aws/aws-sdk-go v1.30.22
	github.com/rancher/lasso v0.0.0-20200513231433-d0ce66327a25
	github.com/rancher/wrangler v0.6.2-0.20200427172034-da9b142ae061
	github.com/rancher/wrangler-api v0.6.1-0.20200427172631-a7c2f09b783e
	github.com/sirupsen/logrus v1.4.2
	k8s.io/api v0.18.0
	k8s.io/apiextensions-apiserver v0.18.0
	k8s.io/apimachinery v0.18.0
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
)
