FROM alpine:3.5
ARG host
ENV cachebuster=223
RUN wget -qO- http://$host:8080/v1/secrets/env/my_env_secret | less