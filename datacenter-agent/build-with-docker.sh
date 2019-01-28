#!/usr/bin/env bash
docker run --rm -v "$GOPATH/src/":/go/src/ -w /go/src/enedification-agents/datacenter-agent golang:1.11 go build -v