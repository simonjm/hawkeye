#!/usr/bin/env bash

GOOS=linux GOARCH=arm GOARM=7 go build
scp hawkeye joe@192.168.11.16:/home/joe/bin
rm -f hawkeye
