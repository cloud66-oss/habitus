#use the golang base image
FROM golang:1.8
MAINTAINER Daniël van Gils

#get govener for package management
#get all the go testing stuff
#Installing Golang-Dep
#copy the source files
RUN go get -u github.com/kardianos/govendor && \
    go get github.com/onsi/ginkgo/ginkgo && \
    go get github.com/onsi/gomega && \
    curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh && \
    mkdir -p /usr/local/go/src/github.com/cloud66-oss/habitus
    
COPY . /usr/local/go/src/github.com/cloud66-oss/habitus

#switch to our app directory
WORKDIR /usr/local/go/src/github.com/cloud66-oss/habitus
