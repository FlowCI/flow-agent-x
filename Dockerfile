FROM ubuntu:18.04

## basic ##
RUN apt update \
    && apt install git curl vim -y \
    && apt install apt-transport-https ca-certificates -y \
    && apt install gnupg-agent software-properties-common -y

## nvm & node ##
ENV NVM_VERSION=v0.34.0
ENV NVM_DIR=/usr/local/nvm
ENV NODE_VERSION=v10.16.3

RUN mkdir $NVM_DIR
RUN bash -c 'curl https://raw.githubusercontent.com/creationix/nvm/$NVM_VERSION/install.sh | bash'
RUN bash -c 'source $NVM_DIR/nvm.sh \
    && nvm install $NODE_VERSION \
    && nvm alias default $NODE_VERSION \
    && nvm use default'

ENV NODE_PATH=$NVM_DIR/$NODE_VERSION/lib/node_modules

## java & maven ##
ENV JAVA_VERSION=openjdk-8-jdk
ENV JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64
ENV MAVEN_VERSION=3.5.4
ENV MAVEN_HOME=/usr/local/apache-maven-$MAVEN_VERSION
ENV M2_HOME=$MAVEN_HOME

RUN apt install $JAVA_VERSION -y \
    && curl -o /usr/local/maven.tar.gz http://apache.mirrors.spacedump.net/maven/maven-3/$MAVEN_VERSION/binaries/apache-maven-$MAVEN_VERSION-bin.tar.gz \
    && tar -C /usr/local -xzf /usr/local/maven.tar.gz

## docker ##
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
RUN add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
RUN apt-get update
RUN apt-get install docker-ce docker-ce-cli containerd.io -y

## docker compose ##
RUN curl -L "https://github.com/docker/compose/releases/download/1.24.1/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose \
    && chmod +x /usr/local/bin/docker-compose \
    && ln -s /usr/local/bin/docker-compose /usr/bin/docker-compose

## start docker ##


## go ##
ENV GOLANG_VERSION=1.12.9
ENV GOROOT=/usr/local/go

RUN curl -o /usr/local/go.tar.gz https://dl.google.com/go/go$GOLANG_VERSION.linux-amd64.tar.gz \
    && tar  -C /usr/local -xzf /usr/local/go.tar.gz

## set PATH ##
ENV PATH=$PATH:$NVM_DIR/versions/node/$NODE_VERSION/bin:$MAVEN_HOME/bin:$GOROOT/bin

ENV TARGET_DIR=/flow.ci.agent
ENV FLOWCI_AGENT_WORKSPACE=${TARGET_DIR}/workspace

RUN mkdir -p $TARGET_DIR \
    && mkdir -p $FLOWCI_AGENT_WORKSPACE

WORKDIR $TARGET_DIR
COPY flow-agent-x $TARGET_DIR

CMD service docker start && ./flow-agent-x