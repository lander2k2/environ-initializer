#!/bin/bash

VERSION=$1

# build go binary
GOOS=linux GOARCH=386 go build -o environ-initializer
BUILD_EXIT=$?
if [[ $BUILD_EXIT != 0 ]]; then
    echo "Error: go build failed"
    exit 1
fi
echo "Go binary built"

# build container and push to registries
IMAGE_ID=$(echo $(docker build --tag quay.io/lander2k2/environ-initializer:$VERSION --quiet .) | awk '{print $NF}')
echo "Container image built, image ID: $IMAGE_ID"
docker tag $IMAGE_ID lander2k2/environ-initializer:$VERSION
docker push quay.io/lander2k2/environ-initializer:$VERSION
docker push lander2k2/environ-initializer:$VERSION
echo "Container images tagged, pushed"

# cleanup
rm environ-initializer
echo "Build complete"

exit 0

