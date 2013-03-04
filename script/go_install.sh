#! /bin/bash

# NOTE for cross compile building from mac run this from $GOROOT/src
#sudo GOOS=linux GOARCH=amd64 CGO_ENABLED=0 ./make.bash --no-clean

VERSION="1.0.3"
echo "install directory? [ENTER](blank for /usr/local/)"
read INSTALL_DIR

set -x

if [ "$INSTALL_DIR" == "" ]
then
    INSTALL_DIR=/usr/local/
fi

echo "Installing go to $INSTALL_DIR"
if [ $(uname) == "Darwin" ]
then
    pushd ~
    curl -O http://go.googlecode.com/files/go{$VERSION}.darwin-amd64.tar.gz
    tar -xzf go1.0.3.darwin-amd64.tar.gz
    rm go1.0.3.darwin-amd64.tar.gz
    sudo mv go $INSTALL_DIR
    pushd $INSTALL_DIR/go/src
    sudo GOOS=linux GOARCH=amd64 CGO_ENABLED=0 ./make.bash --no-clean
    popd
    popd
else
    pushd ~
    curl -O http://go.googlecode.com/files/go1.0.3.linux-amd64.tar.gz
    tar xf go1.0.3.linux-amd64.tar.gz
    rm go1.0.3.linux-amd64.tar.gz
    sudo mv go $INSTALL_DIR
    popd
fi

export GOROOT=$INSTALL_DIR/go
export PATH=$PATH:$GOROOT/bin

unset INSTALL_DIR
