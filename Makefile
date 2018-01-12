# import build configs
env ?= build.env
include $(env)
export $(shell sed 's/=.*//' $(env))

all: build-go build-container release clean

build-go:
	GOOS=linux GOARCH=386 go build -o environ-initializer

build-container:
	docker build --tag $(CONTAINER_REPO):$(CONTAINER_TAG) .

release:
	docker push $(CONTAINER_REPO):$(CONTAINER_TAG)

clean:
	rm environ-initializer

