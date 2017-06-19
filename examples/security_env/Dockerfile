FROM alpine:3.5
ARG host
ARG port
RUN wget -qO- http://$host:$port/v1/secrets/env/my_env_secret | less