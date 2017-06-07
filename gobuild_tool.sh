#!/usr/bin/env bash
go version
go env
echo "########## Install dependency with: go get ##########"
go get github.com/eclipse/paho.mqtt.golang
go get github.com/GaryBoone/GoStats/stats
go get ./...

echo "########## Build tool ##########"
version="1.0.0"
mkdir -p build/$version
for GOOS in darwin linux; do
   for GOARCH in 386 amd64; do
     echo "########## Building mqtt-benchmark-$GOOS-$GOARCH ##########"
     env GOARCH=$GOARCH GOOS=$GOOS go build -v -i -o build/mqtt-benchmark-$GOOS-$GOARCH ./
     cp -f build/mqtt-benchmark-$GOOS-$GOARCH build/$version/mqtt-benchmark-$GOOS-$GOARCH
   done
done

for GOARCH in 386 amd64; do
    echo "########## Building mqtt-benchmark-$GOARCH for Window ##########"
    env GOARCH=$GOARCH GOOS=windows go build -v -i -o build/mqtt-benchmark.$GOARCH.exe ./
    cp -f build/mqtt-benchmark.$GOARCH.exe build/$version/mqtt-benchmark.$GOARCH.exe
done

ls -l build/mqtt-benchmark*
