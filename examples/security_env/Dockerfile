FROM alpine:3.5
ARG host
RUN wget -qO- http://$host:8080/v1/secrets/env/my_env_secret | less