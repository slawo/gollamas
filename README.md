# gollamas
A per model "reverse proxy" which redirects requests to multiple ollama servers.

[![Latest Stable Version](https://flat.badgen.net/github/release/slawo/gollamas/stable)](https://github.com/slawo/gollamas/releases/latest)
[![Licence](https://flat.badgen.net/github/license/slawo/gollamas)](https://github.com/slawo/gollamas/blob/main/LICENSE)
[![CI status](https://flat.badgen.net/github/checks/slawo/gollamas)](https://github.com/slawo/gollamas/actions)
[![docker hub](https://flat.badgen.net/docker/pulls/slawoc/gollamas)](https://hub.docker.com/r/slawoc/gollamas)

This is a reverse proxy for ollama, it accepts mainly chat and generation requests, it reads requests and  transfers the payload to a server which has been specifically assigned to run the model reffered to in the request. Reffer to [API](#api) for a list of endpoints currently supported.

## run binary
Binaries are automatically compiled and made available in the [latest github release](https://github.com/slawo/gollamas/releases/latest).

````
gollamas --level=warn \
    --listen 0.0.0.0:11434 
    --proxy=tinyllama=http://server-01:11434 \
    --proxy=llama3.2-vision=http://server-01:11434 \
    --proxy=deepseek-r1:14b=http://server-02:11434
````
## run on docker
Images are automatically built for `amd64`, `arm`, `arm64`, `riscv64`, `s390x` and `ppc64le`. Issues for other architectures are welcome.

Official images are automaticaly made available on docker hub and ghcr.io. You can run the latest image from either.

### from docker hub
The main images are on [docker hub](https://hub.docker.com/repository/docker/slawoc/gollamas).


```
docker run -it \
  -e GOLLAMAS_PROXIES="llama3.2-vision=http://server:11434,deepseek-r1:14b=http://server2:11434" \
  slawoc/gollamas:latest
```

### github
Alternatively images are published to 
[ghcr.io](https://github.com/slawo/gollamas/pkgs/container/gollamas).

```
docker run -it \
  -e GOLLAMAS_PROXIES="llama3.2-vision=http://server:11434,deepseek-r1:14b=http://server2:11434" \
  ghcr.io/slawo/gollamas:latest
```

## run code locally

````
go run ./*.go --level=trace \
    --listen 0.0.0.0:11434 
    --proxy=tinyllama=http://server-02:11434 \
    --proxy=llama3.2-vision=http://server-02:11434 \
    --proxy=deepseek-r1:14b=http://server-01:11434
````

# Usage

## parameters

The existing flags should remain fairly stable going forward, if flags are to be renamed best effort will be made to keep both the new name and old name as well as existing behaviour until final release.

| Flag | Env var | Description |
|------|---------|-------------|
|`--listen` | "GOLLAMAS_LISTEN", "LISTEN" | address on which the router will be listening on, ie: "localhost:11434" |
| `--proxy value`|  | assigns a destination for a model, can be a url or a connection id ex: --proxy 'llama3.2-vision=http://server:11434' ex: --proxy 'llama3.2-vision=c1 --connection c1=http://server:11434' | `modelName=URL`
|	`--proxies value`| "GOLLAMAS_PROXIES" "PROXIES" | assigns destinations for the models, in the list of model=destination pairs ex: --proxies 'llama3.2-vision=http://server:11434,deepseek-r1:14b=http://server2:11434' |
|	`--connection value`|  | assigns an identifier to a connection which can be reffered to by proxy declarations ex: --connection c1=http://server:11434 --proxy llama=c1 |
|	`--connections value`| "GOLLAMAS_CONNECTIONS" "CONNECTIONS" | provides a list of connections which can be reffered to by id ex: --connections c1=http://server:11434,c2=http://server2:11434 |
|	`--alias value`|  | assigns an alias from an existing model name passed in the proxy configuration 'alias=concrete_model' ex: --alias gpt-3.5-turbo=llama3.2 |
|	`--aliases value`| "GOLLAMAS_ALIASES", "ALIASES" | sets aliases for the given model names ex: --aliases 'gpt-3.5-turbo=llama3.2,deepseek=deepseek-r1:14b' |
|	`--list-aliases`| "GOLLAMAS_LIST_ALIASES" "LIST_ALIASES" | show aliases which match a model when listing models |

## plural flags
You should use the singular flags `--alias`, `--connection` and `--proxy` vs providing a coma separated list to plural flags like `--aliases`, `--connections` and `--proxies`.
Usage of the plural flags is discouraged, those flags have been added as a temporary solution to permit passing the associated environment variables in docker containers. Those flags might be removed in future versions while the environemt variables will be retained.

Setting both sigular flags and plural ones will not result in errors but will result in undefined behaviour which can change with future versions. Use only one type of flags, preferably the singular versions.

## environment variables
For each option you can set either the flags or the environment variables, setting both will result in undefined behavior which can change with future versions.

Use the `GOLLAMAS_` prefixed environment variables.

## connections
You can asign ids to connections like so `--connection CID1=http://main-ai:11434 --connection CID2=http://mini-ai-01:11434` and reffer to each connection by id when listing the models to be proxied `--proxy deepseek-r1:70b=CID1 --proxy tinyllama=CID2`.

When a connection is given an id the the ID will be used instead of the url string in any responses or logs

Since 0.4.1 when multiple models are proxied to the same URL only one connection will be created for that url.It is still possible to create 2 connections on the same URL using the `--connection` flag (`--connection C1=http://server1 --connection C2=http://server1`).

# Features
There are various scenarios this projects attempts to resolve, here is a list of features currently implemented and being considered for implementation:

## Usecases

  - Manage models
    - [x] Map model aliases to existing model names (some tools only allow a pre-defined set of models)
    - [x] Set that by default only the configured models are returned when listing models
    - [x] Set a flag to also return models as aliases
    - [ ] Set option to allow requests to currently running models (ie server has additional model running)
  - [ ] Allow access to models currently running on an instance https://github.com/slawo/gollamas/issues/19
  - [ ] Allow multiple routes to a given model https://github.com/slawo/gollamas/issues/20
  - [ ] preload/keep models in memory https://github.com/slawo/gollamas/issues/22
    - [ ] Preload models (ensure model is loaded uppon startup)
    - [ ] Ping models (maintain model loaded)
    - [ ] Add config to enforce model keep alive globally `"keep_alive": -1` (if it is worth adding functionality for servers without `OLLAMA_KEEP_ALIVE=-1`)
    - [ ] Add config to override model keep alive per model/server `"keep_alive": -1`
  - [ ] Enable fixed context size for models https://github.com/slawo/gollamas/issues/21
    - [ ] Add config to set a default context size (if missing) in each request `"options": { "num_ctx": 4096 }`
    - [ ] Add config to set a default context size (if missing) per model/server `"options": { "num_ctx": 4096 }`
    - [ ] Add config to enforce context size in each request `"options": { "num_ctx": 4096 }`
    - [ ] Add config to enforce context size per model/server `"options": { "num_ctx": 4096 }`

## API
Not all endpoints are covered, particularly endpoints which deal with customisation and creation of models are not supported until there is a clear usecase for this.

  - Supported endpoints
	- [x] `GET /`
	- [x] `GET /api/tags`
	- [x] `GET /api/ps`
	- [x] `GET /api/version`
	- [x] `GET /v1/models`
	- [x] `GET /v1/models/:model`
	- [x] `HEAD /`
	- [x] `HEAD /api/tags`
	- [x] `HEAD /api/version`
	- [x] `POST /api/chat`
	- [x] `POST /api/embed`
	- [x] `POST /api/embeddings`
	- [x] `POST /api/generate`
	- [x] `POST /api/pull`
	- [x] `POST /api/show`
	- [x] `POST /v1/chat/completions`
	- [x] `POST /v1/completions`
	- [x] `POST /v1/embeddings`

  - Not supported
	- [ ] `DELETE /api/delete`
	- [ ] `HEAD /api/blobs/:digest`
	- [ ] `POST /api/blobs/:digest`
	- [ ] `POST /api/copy`
	- [ ] `POST /api/create`
	- [ ] `POST /api/push`

## Internals
The server relies on existing ollama models and middlewares to speed up the development of the initial implementation.
Only the requests which have a `model` ( or the deprecated `name`) field are transfered to the right server.

When possible other endpoints hit all configured servers to either select one answer (ie: the lowest `version` available), or are combined and processed into one response (ie: lists of models).
