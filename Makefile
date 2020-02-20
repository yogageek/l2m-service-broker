# If the USE_SUDO_FOR_DOCKER env var is set, prefix docker commands with 'sudo'
ifdef USE_SUDO_FOR_DOCKER
	SUDO_CMD = sudo
endif

TAG ?= $(shell git describe --tags --always)

PULL ?= IfNotPresent
FPATH = ${GOPATH}/src/advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/cmd/servicemanager

REPO 			= harbor
REPOPROJECT		= frbimo
IMAGE 			?= mongodb-sm
REPOURL			= harbor.arfa.wise-paas.com
REPOPATH 		= $(REPOURL)/$(REPOPROJECT)


build: ## Builds the starter pack
	go build -o bin/$(IMAGE) -i ${FPATH} 

test: ## Runs the tests
	go test -v $(shell go list ./... | grep -v /test/)

linux: ## Builds a Linux executable
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/$(IMAGE) --ldflags="-s" $(FPATH)

image: linux ## Builds a Linux based image
	cp bin/$(IMAGE) image/$(IMAGE)
	$(SUDO_CMD) docker build image/ -t "$(REPOPATH)/$(IMAGE):$(TAG)"

clean: ## Cleans up build artifacts
	rm -f bin/$(IMAGE)
	rm -f image/$(IMAGE)
	rm -f cmd/cronjob/image/mongodb-cronjob

push: image ## Pushes the image to dockerhub, REQUIRES SPECIAL PERMISSION
	$(SUDO_CMD) docker push "$(REPOPATH)/$(IMAGE):$(TAG)"

chart: 
	tar -zcvf bin/tar/mongodb-sm.tar.gz -C charts/ servicemanager/
 

# 
# cronjob area
# 

CPATH = ${GOPATH}/src/advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/cmd/cronjob
CIMAGE ?= mongodb-cronjob
cron-linux: ## Builds a Linux executable
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o cmd/cronjob/image/$(CIMAGE) --ldflags="-s" $(CPATH)

cron-image: cron-linux ## Builds a Linux based image
	$(SUDO_CMD) docker build cmd/cronjob/image/ -t "$(REPOPATH)/$(CIMAGE):$(TAG)"

cron-push: cron-image ## Pushes the image to dockerhub, REQUIRES SPECIAL PERMISSION
	$(SUDO_CMD) docker push "$(REPOPATH)/$(CIMAGE):$(TAG)"
