FROM ubuntu
RUN apt-get update && apt-get install -y wget

ARG host
ARG port
ENV ASSET /asset
RUN wget -q -O $ASSET http://$host:$port/
RUN cat $ASSET
