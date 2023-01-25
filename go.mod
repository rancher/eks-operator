module github.com/rancher/eks-operator

go 1.16

replace k8s.io/client-go => k8s.io/client-go v0.21.2

require (
	github.com/aws/aws-sdk-go v1.44.83
	github.com/blang/semver v3.5.0+incompatible
	github.com/rancher/lasso v0.0.0-20210616224652-fc3ebd901c08
	github.com/rancher/wrangler v0.8.11
	github.com/rancher/wrangler-api v0.6.1-0.20200427172631-a7c2f09b783e
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.7.0
	golang.org/x/net v0.0.0-20220708220712-1185a9018129 // indirect
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8 // indirect
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
)
