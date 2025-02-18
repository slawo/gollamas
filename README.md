# gollamas
A "reverse proxy" for multiple ollama servers running various models.

This is a lowest effort implementation of a reverse proxy for ollama, it accepts all requests which relly on a model and runs the query on a server which has been specifically assigned to run the given model.

## run 

````
go run ./*.go --level=trace --address 0.0.0.0:11434 --proxy=llama3.2-vision=http://server-02:11434 
--proxy=deepseek-r1:14b=http://server-01:11434
````

## API 
implemented:
	- `HEAD /`
	- `GET /`
	- `HEAD /api/tags`
	- `GET /api/tags`
	- `GET /api/ps`
	- `HEAD /api/version`
	- `GET /api/version`
	- `POST /api/pull`
	- `POST /api/generate`
	- `POST /api/chat`
	- `POST /api/embed`
	- `POST /api/embeddings`
	- `POST /api/show`

Not implemented
	- `POST /api/create`
	- `POST /api/push`
	- `POST /api/copy`
	- `DELETE /api/delete`
	- `POST /api/blobs/:digest`
	- `HEAD /api/blobs/:digest`

OpenAI implemented endpoints
	- `GET /v1/models`
	- `GET /v1/models/:model`
	- `POST /v1/chat/completions`
	- `POST /v1/completions`
	- `POST /v1/embeddings`

## Internals
The server relies on existing ollama models and middlewares to speed up the development of the initial implementation.
Only the requests which have a `model` ( or the deprecated `name`) field are transfered to the right server.

Other endpoints hit all servers to either select one answer ie the lowest `version` available, or combined into oone response.
