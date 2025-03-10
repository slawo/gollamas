# gollamas
A "reverse proxy" for multiple ollama servers running various models.

[![Latest Stable Version](https://flat.badgen.net/github/release/slawo/gollamas/stable)](https://github.com/slawo/gollamas/releases/latest)
[![Licence](https://flat.badgen.net/github/license/slawo/gollamas)](https://github.com/slawo/gollamas/blob/main/LICENSE)
[![CI status](https://flat.badgen.net/github/checks/slawo/gollamas)](https://github.com/slawo/gollamas/actions)
[![docker hub](https://flat.badgen.net/docker/pulls/slawoc/gollamas)](https://hub.docker.com/r/slawoc/gollamas)

This is a lowest effort implementation of a reverse proxy for ollama, it takes chat and generation requests and depending on the model in the request it will dispatch the request to a server which has been specifically assigned to run the given model.

## run locally

````
go run ./*.go --level=trace --address 0.0.0.0:11434 --proxy=llama3.2-vision=http://server-02:11434 
--proxy=deepseek-r1:14b=http://server-01:11434
````

## run on docker
Official images are available on docker hub and ghcr.io. You can run the latest image from either: 

  - [docker hub](https://hub.docker.com/repository/docker/slawoc/gollamas): `docker run -it -e GOLLAMAS_PROXIES="llama3.2-vision=http://server:11434,deepseek-r1:14b=http://server2:11434" slawoc/gollamas:latest`
  - [ghcr.io](https://github.com/slawo/gollamas/pkgs/container/gollamas) : `docker run -it -e GOLLAMAS_PROXIES="llama3.2-vision=http://server:11434,deepseek-r1:14b=http://server2:11434" ghcr.io/slawo/gollamas:latest`

# Features

  - Manage models
    - [x] Map model aliases to existing model names (some tools only allow a pre-defined set of models)
    - [x] Set that by default only the configured models are returned when listing models
    - [ ] Set option to allow requests to currently running models (ie server has additional model running)
  - Set model in memory
    - [ ] Preload models (ensure model is loaded uppon startup)
    - [ ] Ping models (maintain model loaded)
    - [ ] Add config to enforce model keep alive globally `"keep_alive": -1`
    - [ ] Add config to override model keep alive per model/server `"keep_alive": -1`
  - Set fixed size context `"options": { "num_ctx": 4096 }`
    - [ ] Add config to set a default context size (if missing) in each request `"options": { "num_ctx": 4096 }`
    - [ ] Add config to set a default context size (if missing) per model/server `"options": { "num_ctx": 4096 }`
    - [ ] Add config to enforce context size in each request `"options": { "num_ctx": 4096 }`
    - [ ] Add config to enforce context size per model/server `"options": { "num_ctx": 4096 }`
  - Proxy API
	- [ ] `DELETE /api/delete`
	- [x] `GET /`
	- [x] `GET /api/tags`
	- [x] `GET /api/ps`
	- [x] `GET /api/version`
	- [x] `GET /v1/models`
	- [x] `GET /v1/models/:model`
	- [x] `HEAD /`
	- [ ] `HEAD /api/blobs/:digest`
	- [x] `HEAD /api/tags`
	- [x] `HEAD /api/version`
	- [ ] `POST /api/blobs/:digest`
	- [x] `POST /api/chat`
	- [ ] `POST /api/copy`
	- [ ] `POST /api/create`
	- [x] `POST /api/embed`
	- [x] `POST /api/embeddings`
	- [x] `POST /api/generate`
	- [x] `POST /api/pull`
	- [x] `POST /api/show`
	- [ ] `POST /api/push`
	- [x] `POST /v1/chat/completions`
	- [x] `POST /v1/completions`
	- [x] `POST /v1/embeddings`

## Internals
The server relies on existing ollama models and middlewares to speed up the development of the initial implementation.
Only the requests which have a `model` ( or the deprecated `name`) field are transfered to the right server.

Other endpoints hit all servers to either select one answer ie the lowest `version` available, or combined into oone response.
