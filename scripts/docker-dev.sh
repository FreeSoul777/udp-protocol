#!/bin/bash
set -e

cd $(dirname "$0")/..
PROJECT_NAME=$(basename $(pwd))
DOCKER_FILE="docker/Dockerfile"
DOCKER_FILE_MD5=$(md5sum $DOCKER_FILE | cut -d' ' -f1)
DOCKER_IMAGE="udp-protocol-dev:$DOCKER_FILE_MD5"

function sync_and_build() {
    mkdir -p "/root/$PROJECT_NAME"
    rsync -av --exclude={'.git','.vscode','bin','*.log'} "/p/$PROJECT_NAME/" "/root/$PROJECT_NAME/"
    cd "/root/$PROJECT_NAME"
    make fmt
    make lint
    make build
}

function run_tests() {
    cd "/root/$PROJECT_NAME"
    make test-all
}

if [ -f /.dockerenv ]; then
    sync_and_build
    run_tests
    bash -l
else
    if [ "$(docker images -q "$DOCKER_IMAGE" 2> /dev/null)" == "" ]; then
        echo "Building Docker image $DOCKER_IMAGE..."
        docker build --network=host . -t "$DOCKER_IMAGE" -f "$DOCKER_FILE"
    fi

    docker run \
        -v "$(pwd):/p/$PROJECT_NAME" \
        --rm \
        -it \
        "$DOCKER_IMAGE" \
        /bin/bash -c "PROJECT_NAME=$PROJECT_NAME; $(declare -f sync_and_build); $(declare -f run_tests); sync_and_build; run_tests; bash -l"
fi