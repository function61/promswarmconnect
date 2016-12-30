# this Dockerfile is only for setting up a container in which to compile docker-prometheus-bridge

FROM ubuntu:xenial-20160818

RUN apt-get update && apt-get install -y curl git

RUN curl --fail --location https://storage.googleapis.com/golang/go1.7.4.linux-amd64.tar.gz | tar -C /usr/local -xzf -

# http://stackoverflow.com/questions/38024160/how-to-get-etc-profile-to-run-automatically-in-alpine-docker
# 
# /etc/profile not sourced, so add to root's profile
# ok, /root/.profile not sourced either

# RUN echo "GOPATH=/app" >> /etc/environment \
# 	&& echo "PATH=\$PATH:/usr/local/go/bin" >> /etc/environment

ENV GOPATH=/app
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/go/bin

# docker run -it --rm -v "$(pwd):/app" -v "/run/docker.sock:/run/docker.sock" gosandbox

WORKDIR /app

CMD bash
