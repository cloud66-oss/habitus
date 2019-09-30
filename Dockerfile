FROM golang:1.12 AS tester
MAINTAINER Cloud 66 Engineering

RUN apt-get update -y && apt-get upgrade -y

ENV APP_DIR=$GOPATH/src/github.com/cloud66-oss/habitus
RUN mkdir -p $APP_DIR
COPY . $APP_DIR
WORKDIR $APP_DIR

CMD ["go", "test", ".", "./configuration"]

FROM tester AS builder

RUN go get github.com/mitchellh/gox
RUN mkdir -p /usr/local/go/src/github.com/cloud66-oss/habitus
COPY . /usr/local/go/src/github.com/cloud66-oss/habitus

#switch to our app directory
WORKDIR /usr/local/go/src/github.com/cloud66-oss/habitus

RUN ./compile.sh `git describe --abbrev=0 --tags`
