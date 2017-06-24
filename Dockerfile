FROM golang:1.8-alpine
VOLUME /in
COPY hawkeye /go/bin/hawkeye
CMD ["hawkeye", "-out-dir", "/in/done", "/in"]

