#! /bin/bash 

VERSION=$(cat .version)
wget https://github.com/oscar-martin/tail_folders/releases/download/${VERSION}/tail_folders_linux_amd64
docker login -u "$DOCKER_USERNAME" -p "$DOCKER_PASSWORD"
docker build --build-args VERSION="${VERSION}" -t oscarmartin/tail_folders:"${VERSION}" .
docker push oscarmartin/tail_folders:"${VERSION}"