

apt install make build-essential libssl-dev zlib1g-dev libbz2-dev libreadline-dev libsqlite3-dev -y


## pyenv ##
RUN curl https://pyenv.run | bash
RUN echo 'export PATH="$HOME/.pyenv/bin:$PATH"' >> ${HOME}/.profile
RUN echo 'eval "$(pyenv init -)"' >> ${HOME}/.profile
RUN echo 'eval "$(pyenv virtualenv-init -)"' >> ${HOME}/.profile

## python 3.6.10 ##
RUN $HOME/.pyenv/bin/pyenv install 3.6.10
RUN $HOME/.pyenv/bin/pyenv global 3.6.10
ENV $FLOWCI_AGENT_PY_ROOT=$HOME/.pyenv/versions/3.6.10/bin

export PYENV_ROOT=/ws/.pyenv
export PATH=$PYENV_ROOT/bin:$PATH
eval "$(pyenv init -)"
pyenv rehash

# FLOWCI_PY_ROOT