FROM mcr.microsoft.com/devcontainers/go:1.23
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && sudo apt-get -y install git ca-certificates curl && install -m 0755 -d /etc/apt/keyrings \
    && curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc \
    && chmod a+r /etc/apt/keyrings/docker.asc
RUN  echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian \
    $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
    sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && sudo apt-get -y install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
RUN chown -R vscode /go
