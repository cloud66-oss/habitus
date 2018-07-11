#use the golang base image
FROM golang:1.8
MAINTAINER Daniël van Gils

RUN go get github.com/mitchellh/gox

#copy the source files
RUN mkdir -p /usr/local/go/src/github.com/cloud66-oss/habitus
COPY . /usr/local/go/src/github.com/cloud66-oss/habitus

#switch to our app directory
WORKDIR /usr/local/go/src/github.com/cloud66-oss/habitus

RUN ./compile.sh `git describe --abbrev=0 --tags`
