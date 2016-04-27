#!/usr/bin/env bash

GOOS=linux GOARCH=arm GOARM=6 go build
scp hawkeye joe@192.168.11.2:/home/joe/bin
rm -f hawkeye
