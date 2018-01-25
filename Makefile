release: build-go build-image push clean

test: build-go build-image-test push-test clean e2e-test

build-go:
	GOOS=linux GOARCH=386 go build -o environ-initializer

build-image:
	./check_env.sh IMAGE_REPO
	./check_env.sh IMAGE_TAG
	docker build --tag $(IMAGE_REPO):$(IMAGE_TAG) .

build-image-test:
	./check_env.sh IMAGE_REPO
	docker build --tag $(IMAGE_REPO):test .

push:
	./check_env.sh IMAGE_REPO
	./check_env.sh IMAGE_TAG
	docker push $(IMAGE_REPO):$(IMAGE_TAG)

push-test:
	./check_env.sh IMAGE_REPO
	docker push $(IMAGE_REPO):test

clean:
	rm environ-initializer

e2e-test:
	./check_env.sh KUBECONFIG
	./check_env.sh IMAGE_REPO
	go test -v ./test/ -args -kubeconfig $(KUBECONFIG) -repo $(IMAGE_REPO) -goblin.timeout 180s

