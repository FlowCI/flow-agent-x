FROM ubuntu:18.04

RUN rm /bin/sh && ln -s /bin/bash /bin/sh

## basic ##
RUN apt update; \
    apt install curl -y

## git ##
RUN apt install git -y

## npm & nodejs ##
ENV NVM_VERSION=v0.34.0
ENV DEFAULT_NVM_DIR=/root/.nvm
ENV NODE_VERSION=v10.16.3

# Install nvm with node and npm
RUN curl https://raw.githubusercontent.com/creationix/nvm/$NVM_VERSION/install.sh | bash \
    && source $DEFAULT_NVM_DIR/nvm.sh \
    && nvm install $NODE_VERSION \
    && nvm alias default $NODE_VERSION \
    && nvm use default

## java & maven ##
## go ##
## docker & docker-compose ##

ENV TARGET_DIR=/work
RUN mkdir -p $TARGET_DIR
COPY flow-agent-x $TARGET_DIR