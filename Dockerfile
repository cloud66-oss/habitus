#use the golang base image
FROM golang:1.6
MAINTAINER Daniël van Gils

#setup package manager
RUN go get github.com/tools/godep

#get all the go testing stuff
RUN go get github.com/onsi/ginkgo/ginkgo
RUN go get github.com/onsi/gomega

#stuff for cross compiling
RUN go get github.com/mitchellh/gox

#copy the source files
RUN mkdir -p /usr/local/go/src/github.com/cloud66/habitus
ADD . /usr/local/go/src/github.com/cloud66/habitus

#switch to our app directory
WORKDIR /usr/local/go/src/github.com/cloud66/habitus