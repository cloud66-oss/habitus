FROM ubuntu:16.04
RUN apt-get update
RUN apt-get install curl -y
RUN apt-get install less -y
ARG habitus_host
ARG habitus_port
ARG habitus_password
ARG habitus_user
RUN echo http://$habitus_host:$habitus_port/v1/secrets/env/my_env_secret
RUN echo $habitus_user:$habitus_password
RUN curl -s -u $habitus_user:$habitus_password http://$habitus_host:$habitus_port/v1/secrets/env/my_env_secret