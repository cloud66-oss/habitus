FROM ubuntu:16.04
RUN apt-get update
RUN apt-get install wget -y
RUN apt-get install less -y
ARG habitus_host
ARG habitus_port
ARG habitus_password
ARG habitus_user
RUN wget --http-user=$habitus_user --http-password=$habitus_password -qO- http://$habitus_host:$habitus_port/v1/secrets/env/my_env_secret | less