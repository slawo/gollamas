FROM mcr.microsoft.com/devcontainers/go:1.23
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
  && sudo apt-get -y install git
RUN chown -R vscode /go
