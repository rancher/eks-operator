SEVERITIES = HIGH,CRITICAL

.PHONY: all
all:
	sudo docker build --build-arg TAG=$(TAG) -t rancher/eks-operator:$(TAG) .

.PHONY: image-push
image-push:
	docker push rancher/eks-operator:$(TAG) >> /dev/null

.PHONY: scan
image-scan:
	trivy --severity $(SEVERITIES) --no-progress --skip-update --ignore-unfixed rancher/eks-operator:$(TAG)

.PHONY: image-manifest
image-manifest:
	docker image inspect rancher/eks-operator:$(TAG)
	DOCKER_CLI_EXPERIMENTAL=enabled docker manifest create rancher/eks-operator:$(TAG) \
		$(shell docker image inspect rancher/eks-operator:$(TAG) | jq -r '.[] | .RepoDigests[0]')
