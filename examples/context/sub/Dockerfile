#use the golang base image
FROM golang:1.7
LABEL MAINTAINER="Daniël van Gils"
#get all the go testing stuff
ARG BUILD_ARGUMENT
ENV AWESOME_ENVIRONMENT=$BUILD_ARGUMENT

COPY entrypoint.sh /entrypoint.sh

CMD ["sh","/entrypoint.sh"]
