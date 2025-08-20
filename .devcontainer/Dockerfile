FROM rockylinux:8.5 as base

ARG REPO_NAME=langchaingo-play
ARG USERNAME=playa
ARG USER_UID=1000
ARG USER_GID=$USER_UID

# ca certs
COPY cas.pem /etc/pki/ca-trust/source/anchors/cas.pem 
RUN update-ca-trust

RUN set -eux -o pipefail && \
    dnf install -y nss-tools openssl ca-certificates && \
    dnf install -y curl dnf-utils dnf-plugins-core epel-release findutils git sudo vim && \
    dnf config-manager --set-enabled powertools 

FROM base as dev

# docker
RUN set -eux -o pipefail && \
    yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo && \
    yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin expect
# dev tools
RUN set -eux -o pipefail && \
    dnf install -y jq lsof make procps-ng tree 

FROM dev as user

# Create the user
RUN groupadd --gid ${USER_GID} ${USERNAME} && \
  useradd --uid ${USER_UID} --gid ${USER_GID} -m ${USERNAME} -s /bin/zsh && \
  echo ${USERNAME} ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/${USERNAME} && \
  chmod 0440 /etc/sudoers.d/${USERNAME}

########### NON ROOT FROM HERE DOWN ###########
USER ${USERNAME}

# pretty bash
COPY --chown=${USERNAME}:${USERNAME} .bash_pretty ${HOME}/.bash_pretty
RUN set -eux && \
    echo "source ${HOME}/.bash_pretty" >> ${HOME}/.bashrc

RUN set -eux && \
    mkdir -p ${HOME}/bin && \
    sudo mkdir -p /workdir/${REPO_NAME} && \
    sudo chown ${USERNAME}:${USERNAME} /workdir
    
# envs    
ENV HOME "/home/${USERNAME}"
ENV SDK_HOME ${HOME}/sdk
ENV PATH ${HOME}/bin:${PATH}

# go
ENV GO_VERSION "1.25.0"
ENV GOPATH ${SDK_HOME}/go
ENV PATH ${GOPATH}/bin:${HOME}/sdk/go:${PATH}
RUN set -eux && \
    export GO_ARCH="$(arch | sed 's/aarch64/arm64/' | sed 's/x86_64/amd64/')" && \
    cd /tmp && mkdir -p ${SDK_HOME} && \
    curl -k -o go.tar.gz -L https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz && \
    tar xzf go.tar.gz && mv go ${SDK_HOME} && rm /tmp/go.tar.gz && \
    cd ${SDK_HOME} && mv go go${GO_VERSION} && ln -s go${GO_VERSION} go

# ollama serve & && ollama run llama3.2 # see scripts/ollama-serve-model
ENV MODEL="llama3.2"
RUN set -eux && \
    curl -fsSL https://ollama.com/install.sh | sh
    
# gemini-cli (set GEMINI_API_KEY for higher rate limit)
ENV GEMINI_API_KEY=""
ENV NVM_DIR "${HOME}/.nvm"
RUN mkdir -p ${NVM_DIR} && \
    curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.3/install.sh | bash && \
    source "$NVM_DIR/nvm.sh" && \
    nvm install 22 && \
    echo "source $NVM_DIR/nvm.sh" >> "$HOME/.bash_npm" && \
    echo "source $NVM_DIR/bash_completion" >> "$HOME/.bash_npm" && \
    echo "nvm install 22" >> "$HOME/.bash_npm" && \
    echo "source ${HOME}/.bash_npm" >> ${HOME}/.bashrc && \
    npm install -g @google/gemini-cli && \
    npm install -g @anthropic-ai/claude-code