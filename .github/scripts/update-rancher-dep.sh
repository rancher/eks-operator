#!/bin/bash
#
# Submit new EKS operator version against rancher/rancher

set -ue

NEW_EKS_OPERATOR_VERSION="$1"  # e.g. 1.1.0-rc2

if [ -z "${GITHUB_WORKSPACE:-}" ]; then
    RANCHER_DIR="$(dirname -- "$0")/../../../rancher"
else
    RANCHER_DIR="${GITHUB_WORKSPACE}/rancher"
fi


if [ ! -e ~/.gitconfig ]; then
    git config --global user.name "highlander-ci-bot"
    git config --global user.email "highlander-ci@proton.me"
fi

cd "${RANCHER_DIR}"
go get "github.com/rancher/eks-operator@v${NEW_EKS_OPERATOR_VERSION}"
go mod tidy
cd pkg/apis
go get "github.com/rancher/eks-operator@v${NEW_EKS_OPERATOR_VERSION}"
go mod tidy
cd ../../
git add go.* pkg/apis/go.*

git commit -m "Updating to EKS operator v${NEW_EKS_OPERATOR_VERSION}"

