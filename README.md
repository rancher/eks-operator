# rancher/eks-operator

EKS operator is a Kubernetes CRD controller that controls cluster provisioning in Elastic Kubernetes Service using an EKSClusterConfig defined by a Custom Resource Definition.

## Build

    TAG=master make

## Develop

The easiest way to debug and develop the EKS operator is to replace the default operator on a running Rancher instance with your local one.

* Run a local Rancher server
* Provision an EKS cluster
* Scale the eks-operator deployment to replicas=0 in the Rancher UI
* Open the eks-operator repo in Goland, set `KUBECONFIG=<kubeconfig_path>` in Run Configuration Environment
* Run the eks-operator in Debug Mode
* Set breakpoints

## Release

#### When should I release?

A KEv2 operator should be released if

* There have been several commits since the last release,
* You need to pull in an update/bug fix/backend code to unblock UI for a feature enhancement in Rancher
* The operator needs to be unRC for a Rancher release

#### How do I release?

Tag the latest commit on the `master` branch. For example, if latest tag is `v1.1.6-rc1` you would tag `v1.1.6-rc2`.

    git pull upstream master --tags     // get the latest upstream changes (not your fork)
    git tag v1.0.8-rc2                  // tag HEAD
    git push upstream v1.0.8-rc2        // push the tag

Submit a [rancher/charts PR](https://github.com/rancher/charts/pull/2242) to update the operator and operator-crd chart versions.
Submit a [rancher/rancher PR](https://github.com/rancher/rancher/pull/39745) to update the bundled chart.

#### How do I unRC?

UnRC is the process of removing the rc from a KEv2 operator tag and means the released version is stable and ready for use. Release the KEv2 operator but instead of bumping the rc, remove the rc. For example, if the latest release of EKS operator is `v1.1.6-rc1`, release the next version without the rc which would be `v1.1.6`.
