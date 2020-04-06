FROM flowci/agent-base:1.0

## default work dir
ENV FLOWCI_AGENT_WORKSPACE=/ws
RUN mkdir -p $FLOWCI_AGENT_WORKSPACE

WORKDIR $FLOWCI_AGENT_WORKSPACE
COPY ./bin/flow-agent-x-linux /usr/bin

## start docker ##
CMD service docker start && flow-agent-x-linux