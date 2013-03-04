#!/bin/bash

#set -x

GP=$GOPATH
export GOPATH=$(pwd)

export OLD_GOARCH=$GOARCH
export OLD_GOOS=$GOOS

echo "Building for linux...."
GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -o bin/failover_linux main.go 
#GOARCH=amd64 GOOS=linux go build -o bin/failover_linux main.go
echo "Done building linux"

echo "Building for mac...."
GOARCH=amd64 GOOS=darwin go build -o bin/failover_mac main.go 
echo "Done building mac"

export GOARCH=$OLD_GOARCH
export GOOS=$OLD_GOOS

echo "Resetting go path $GOPATH"
export GOPATH=$GP;
echo "back to $GOPATH"

