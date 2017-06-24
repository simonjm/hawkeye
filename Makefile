IMAGE=hawkeye

build:
	GOOS=linux go build
	docker build -t $(IMAGE) .

run: build
	docker run --rm --name hawkeye --init -v $(shell pwd)/in:/in $(IMAGE)

clean:
	rm -f hawkeye

.PHONY: clean