SEVERITIES = HIGH,CRITICAL

.PHONY: all
all:
	sudo docker build --build-arg TAG=$(TAG) -t rancher/eks-controller:$(TAG) .

.PHONY: image-push
image-push:
	docker push rancher/eks-controller:$(TAG) >> /dev/null

.PHONY: scan
image-scan:
	trivy --severity $(SEVERITIES) --no-progress --skip-update --ignore-unfixed rancher/eks-controller:$(TAG)

.PHONY: image-manifest
image-manifest:
	docker image inspect rancher/eks-controller:$(TAG)
	DOCKER_CLI_EXPERIMENTAL=enabled docker manifest create rancher/eks-controller:$(TAG) \
		$(shell docker image inspect rancher/eks-controller:$(TAG) | jq -r '.[] | .RepoDigests[0]')
