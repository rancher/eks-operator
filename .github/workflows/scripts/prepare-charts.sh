#!/bin/bash
# Prepares the rancher-eks-operator charts for the ship-family-staged action
# (charts v2, cargueiro) from a release tag. The output matches what `make charts`
# gave rancher/charts until now. The action stamps the final chart version.
#
# Usage: prepare-charts.sh <tag> <output dir>   (run from the repository root)
set -euo pipefail

TAG=${1:?usage: $0 <tag> <output dir>}
OUT_DIR=${2:?usage: $0 <tag> <output dir>}

# The tags of eks-operator are v1.N.P and v1.N.P-rc.R, the prerelease form that
# cargueiro accepts.
if ! [[ "$TAG" =~ ^v1\.([0-9]+)\.[0-9]+(-rc\.[0-9]+)?$ ]]; then
  echo "Error: tag $TAG must look like v1.N.P or v1.N.P-rc.R" >&2
  exit 1
fi
TAG_MINOR=${BASH_REMATCH[1]}
APP_VERSION=${TAG#v}

SRC_MAIN=charts/eks-operator
SRC_CRD=charts/eks-operator-crd
for src in "$SRC_MAIN" "$SRC_CRD"; do
  if [ ! -f "$src/Chart.yaml" ]; then
    echo "Error: $src/Chart.yaml not found, run this script from the repository root" >&2
    exit 1
  fi
done

# Tag v1.N.x is for Rancher 2.N, but cargueiro takes the chart major from the
# rancher-version annotation. Refuse a mismatch before anything is pushed.
CHART_MINOR=$(sed -n -E 's|^[[:space:]]+catalog\.cattle\.io/rancher-version:[^0-9]*2\.([0-9]+)\..*|\1|p' "$SRC_MAIN/Chart.yaml")
if [ "$CHART_MINOR" != "$TAG_MINOR" ]; then
  echo "Error: tag $TAG is for Rancher 2.${TAG_MINOR}, but the catalog.cattle.io/rancher-version annotation in" >&2
  echo "$SRC_MAIN/Chart.yaml is for Rancher 2.${CHART_MINOR:-?}. Update the rancher-version and kube-version" >&2
  echo "annotations on this branch before tagging." >&2
  exit 1
fi

MAIN_DIR="$OUT_DIR/rancher-eks-operator"
CRD_DIR="$OUT_DIR/rancher-eks-operator-crd"
rm -rf "$MAIN_DIR" "$CRD_DIR"
mkdir -p "$OUT_DIR"

# The operator chart: the appVersion, the image of the helm.sh/images annotation
# and the image tag. Only the tag right after the eks-operator repository.
cp -r "$SRC_MAIN" "$MAIN_DIR"
sed -i.bak \
  -e "s|^appVersion:.*|appVersion: ${APP_VERSION}|" \
  -e "s|image: rancher/eks-operator:.*|image: rancher/eks-operator:${TAG}|" \
  "$MAIN_DIR/Chart.yaml"
sed -i.bak -e "/repository: rancher\/eks-operator$/{n;s|tag: .*|tag: ${TAG}|;}" "$MAIN_DIR/values.yaml"

cp -r "$SRC_CRD" "$CRD_DIR"
sed -i.bak -e "s|^appVersion:.*|appVersion: ${APP_VERSION}|" "$CRD_DIR/Chart.yaml"

find "$MAIN_DIR" "$CRD_DIR" -name '*.bak' -delete

# Fail when an edit did not apply, as it would stage a chart that still has the
# placeholders of the source.
for dir in "$MAIN_DIR" "$CRD_DIR"; do
  if ! grep -qxF "appVersion: ${APP_VERSION}" "$dir/Chart.yaml"; then
    echo "Error: could not set 'appVersion: ${APP_VERSION}' in $dir/Chart.yaml" >&2
    exit 1
  fi
done
if ! grep -qF "image: rancher/eks-operator:${TAG}" "$MAIN_DIR/Chart.yaml"; then
  echo "Error: could not set the image 'rancher/eks-operator:${TAG}' in $MAIN_DIR/Chart.yaml" >&2
  exit 1
fi
if ! grep -A1 'repository: rancher/eks-operator$' "$MAIN_DIR/values.yaml" | grep -qF "tag: ${TAG}"; then
  echo "Error: could not set the eks-operator tag in $MAIN_DIR/values.yaml" >&2
  exit 1
fi
