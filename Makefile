IMAGE=hawkeye

build:
	GOOS=linux go build
	docker build -t $(IMAGE) .

run: build
	docker run --rm --name hawkeye --init -v $(shell pwd)/in:/in $(IMAGE)

deploy:
	GOOS=linux GOARCH=arm GOARM=7 go build
	scp hawkeye joe@192.168.0.54:/home/joe/bin
	rm -f hawkeye

clean:
	rm -f hawkeye

.PHONY: clean