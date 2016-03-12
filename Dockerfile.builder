#use the golang base image
FROM golang:1.6
MAINTAINER Daniël van Gils

#copy the source files
RUN mkdir -p /usr/local/go/src/github.com/cloud66/habitus
ADD . /usr/local/go/src/github.com/cloud66/habitus

#switch to our app directory
WORKDIR /usr/local/go/src/github.com/cloud66/habitus

RUN go build