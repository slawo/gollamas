# gollamas
A "reverse proxy" for multiple ollama servers running various models.

This is a lowest effort implementation of a reverse proxy for ollama, it accepts chat and generation requests which relly on a model and runs the query on a server which has been specifically assigned to run the given model.

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

  - [ ] Preload models (ensure model is loaded uppon startup)
  - [ ] Ping models (maintain model loaded)
  - Fixed size context
    - [ ] Configure global context size
    - [ ] Enforce context size in each request
    - [ ] Configure context size per model/server
  - Proxy API
	- [ ] `DELETE /api/delete`
	- [V] `GET /`
	- [V] `GET /api/tags`
	- [V] `GET /api/ps`
	- [V] `GET /api/version`
	- [V] `GET /v1/models`
	- [V] `GET /v1/models/:model`
	- [V] `HEAD /`
	- [ ] `HEAD /api/blobs/:digest`
	- [V] `HEAD /api/tags`
	- [V] `HEAD /api/version`
	- [ ] `POST /api/blobs/:digest`
	- [V] `POST /api/chat`
	- [ ] `POST /api/copy`
	- [ ] `POST /api/create`
	- [V] `POST /api/embed`
	- [V] `POST /api/embeddings`
	- [V] `POST /api/generate`
	- [V] `POST /api/pull`
	- [V] `POST /api/show`
	- [ ] `POST /api/push`
	- [V] `POST /v1/chat/completions`
	- [V] `POST /v1/completions`
	- [V] `POST /v1/embeddings`

## Internals
The server relies on existing ollama models and middlewares to speed up the development of the initial implementation.
Only the requests which have a `model` ( or the deprecated `name`) field are transfered to the right server.

Other endpoints hit all servers to either select one answer ie the lowest `version` available, or combined into oone response.
