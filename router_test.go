package main_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ollama/ollama/api"
	gollamas "github.com/slawo/gollamas"
	"github.com/slawo/gollamas/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewRouterFailsOnMissingConfig(t *testing.T) {
	r, err := gollamas.NewRouter(nil)
	assert.EqualError(t, err, "missing ollama client map")
	assert.Nil(t, r)
}

func TestNewRouterFailsOnEmptyConfig(t *testing.T) {
	r, err := gollamas.NewRouter(map[string]gollamas.IOllamaClient{})
	assert.EqualError(t, err, "empty ollama client map")
	assert.Nil(t, r)
}

func TestNewRouterFailsOnNilClient(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	r, err := gollamas.NewRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": nil,
	})
	assert.EqualError(t, err, "nil client for model other_model")
	assert.Nil(t, r)
}

func TestNewRouterFailOnInvalidAlias(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	r, err := gollamas.NewRouter(map[string]gollamas.IOllamaClient{
		"llama3.2": c1,
	}, gollamas.WithAlias("llama3", "wrong_model"))
	assert.EqualError(t, err, "alias llama3 points to unknown model wrong_model")
	assert.Nil(t, r)
	c1.AssertExpectations(t)
}

func TestNewRouter(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	r, err := gollamas.NewRouter(map[string]gollamas.IOllamaClient{
		"llama3.2": c1,
	}, gollamas.WithAlias("llama3", "llama3.2"))
	assert.NoError(t, err)
	assert.NotNil(t, r)
	c1.AssertExpectations(t)
}

func newRouter(cmap map[string]gollamas.IOllamaClient, opts ...gollamas.RouterOption) (context.Context, context.CancelFunc, *gollamas.Router, error) {
	ctx, cancel := context.WithCancel(context.Background())
	r, err := gollamas.NewRouter(cmap, opts...)
	return ctx, cancel, r, err
}

func TestRouterChat(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	}, gollamas.WithAlias("llama3", "llama3.2"), gollamas.WithAlias("some_alias", "other_model"))
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)
	cb := func(api.ChatResponse) error { return nil }

	err = r.Chat(ctx, &api.ChatRequest{
		Model: "unknown_model",
	}, cb)
	assert.EqualError(t, err, "gollamas router is missing a valid route to model unknown_model")

	req := &api.ChatRequest{
		Model: "llama3.2",
	}
	c1.On("Chat", ctx, req, mock.Anything).Once().Return(nil)
	err = r.Chat(ctx, req, cb)
	assert.NoError(t, err)

	reqAlias := &api.ChatRequest{
		Model: "llama3",
	}
	c1.On("Chat", ctx, req, mock.Anything).Once().Return(nil)
	err = r.Chat(ctx, reqAlias, cb)
	assert.NoError(t, err)

	req = &api.ChatRequest{
		Model: "llama3.2:latest",
	}
	c1.On("Chat", ctx, req, mock.Anything).Once().Return(nil)
	err = r.Chat(ctx, req, cb)
	assert.NoError(t, err)

	req = &api.ChatRequest{
		Model: "other_model",
	}
	c2.On("Chat", ctx, req, mock.Anything).Once().Return(nil)
	err = r.Chat(ctx, req, cb)
	assert.NoError(t, err)

	reqAlias = &api.ChatRequest{
		Model: "some_alias",
	}
	c2.On("Chat", ctx, req, mock.Anything).Once().Return(nil)
	err = r.Chat(ctx, reqAlias, cb)
	assert.NoError(t, err)

	c2.On("Chat", ctx, req, mock.Anything).Once().Return(errors.New("some error"))
	err = r.Chat(ctx, req, cb)
	assert.EqualError(t, err, "some error")

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}

func TestRouterCopy(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	}, gollamas.WithAlias("llama3", "llama3.2"), gollamas.WithAlias("some_alias", "other_model"))
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	req := &api.CopyRequest{
		Source:      "llama3.2",
		Destination: "llama3-backup",
	}
	err = r.Copy(ctx, req)
	assert.EqualError(t, err, "gollamas: router doesn't support copying models")

	req = &api.CopyRequest{
		Source:      "other_model",
		Destination: "other_model-backup",
	}
	err = r.Copy(ctx, req)
	assert.EqualError(t, err, "gollamas: router doesn't support copying models")

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}

func TestRouterCreate(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	}, gollamas.WithAlias("llama3", "llama3.2"), gollamas.WithAlias("some_alias", "other_model"))
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	cb := func(api.ProgressResponse) error { return nil }

	req := &api.CreateRequest{
		Model: "llama3.2",
	}
	err = r.Create(ctx, req, cb)
	assert.EqualError(t, err, "gollamas: router doesn't support creating models")

	req = &api.CreateRequest{
		Model: "other_model",
	}
	err = r.Create(ctx, req, cb)
	assert.EqualError(t, err, "gollamas: router doesn't support creating models")

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}

// CreateBlob(ctx context.Context, digest string, r io.Reader) error
func TestRouterCreateBlob(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	}, gollamas.WithAlias("llama3", "llama3.2"), gollamas.WithAlias("some_alias", "other_model"))
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	err = r.CreateBlob(ctx, "llama3.2", strings.NewReader("some data"))
	assert.EqualError(t, err, "gollamas: router doesn't like blobs (not supported)")

	err = r.CreateBlob(ctx, "other_model", strings.NewReader("some data"))
	assert.EqualError(t, err, "gollamas: router doesn't like blobs (not supported)")

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}

// Delete(ctx context.Context, req *api.DeleteRequest) error
func TestRouterDelete(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	}, gollamas.WithAlias("llama3", "llama3.2"), gollamas.WithAlias("some_alias", "other_model"))
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	req := &api.DeleteRequest{
		Model: "llama3.2",
	}
	err = r.Delete(ctx, req)
	assert.EqualError(t, err, "gollamas: router doesn't support deleting models")

	req = &api.DeleteRequest{
		Model: "other_model",
	}
	err = r.Delete(ctx, req)
	assert.EqualError(t, err, "gollamas: router doesn't support deleting models")

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}

// Embed(ctx context.Context, req *api.EmbedRequest) (*api.EmbedResponse, error)
func TestRouterEmbed(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	}, gollamas.WithAlias("llama3", "llama3.2"), gollamas.WithAlias("some_alias", "other_model"))
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	res, err := r.Embed(ctx, &api.EmbedRequest{
		Model: "unknown_model",
		Input: "Why is the sky blue?",
	})
	assert.EqualError(t, err, "gollamas router is missing a valid route to model unknown_model")
	assert.Nil(t, res)

	req := &api.EmbedRequest{
		Model: "llama3.2",
		Input: "Why is the sky blue?",
	}
	out1 := &api.EmbedResponse{
		Model: "llama3.2",
		Embeddings: [][]float32{{
			0.010071029, -0.0017594862, 0.05007221, 0.04692972, 0.054916814,
			0.008599704, 0.105441414, -0.025878139, 0.12958129, 0.031952348,
		}},
	}
	c1.On("Embed", ctx, req).Once().Return(out1, nil)
	res, err = r.Embed(ctx, req)
	assert.NoError(t, err)
	assert.EqualValues(t, out1, res)

	reqAlias := &api.EmbedRequest{
		Model: "llama3",
		Input: "Why is the sky blue?",
	}
	c1.On("Embed", ctx, req).Once().Return(out1, nil)
	res, err = r.Embed(ctx, reqAlias)
	assert.NoError(t, err)
	assert.EqualValues(t, out1, res)

	req = &api.EmbedRequest{
		Model: "other_model",
		Input: "Why is the sky blue?",
	}
	out2 := &api.EmbedResponse{
		Model: "other_model",
		Embeddings: [][]float32{{
			0.010071029, -0.0017594862, 0.05007221, 0.04692972, 0.054916814,
			0.008599704, 0.105441414, -0.025878139, 0.12958129, 0.031952348,
		}},
	}
	c2.On("Embed", ctx, req).Once().Return(out2, nil)
	res, err = r.Embed(ctx, req)
	assert.NoError(t, err)
	assert.EqualValues(t, out2, res)

	reqAlias = &api.EmbedRequest{
		Model: "some_alias",
		Input: "Why is the sky blue?",
	}
	c2.On("Embed", ctx, req).Once().Return(out2, nil)
	res, err = r.Embed(ctx, reqAlias)
	assert.NoError(t, err)
	assert.EqualValues(t, out2, res)

	req = &api.EmbedRequest{
		Model: "other_model",
		Input: "Not a sensible request",
	}
	c2.On("Embed", ctx, req).Once().Return(nil, errors.New("some error"))
	res, err = r.Embed(ctx, req)
	assert.EqualError(t, err, "some error")
	assert.Nil(t, res)

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}

// Embeddings(ctx context.Context, req *api.EmbeddingRequest) (*api.EmbeddingResponse, error)
func TestRouterEmbeddings(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	}, gollamas.WithAlias("llama3", "llama3.2"), gollamas.WithAlias("some_alias", "other_model"))
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	res, err := r.Embeddings(ctx, &api.EmbeddingRequest{
		Model:  "unknown_model",
		Prompt: "Why is the sky blue?",
	})
	assert.EqualError(t, err, "gollamas router is missing a valid route to model unknown_model")
	assert.Nil(t, res)

	req := &api.EmbeddingRequest{
		Model:  "llama3.2",
		Prompt: "Why is the sky blue?",
	}
	out1 := &api.EmbeddingResponse{
		Embedding: []float64{
			0.5670403838157654, 0.009260174818336964, 0.23178744316101074, -0.2916173040866852, -0.8924556970596313,
			0.8785552978515625, -0.34576427936553955, 0.5742510557174683, -0.04222835972905159, -0.137906014919281,
		},
	}
	c1.On("Embeddings", ctx, req).Once().Return(out1, nil)
	res, err = r.Embeddings(ctx, req)
	assert.NoError(t, err)
	assert.EqualValues(t, out1, res)

	reqAlias := &api.EmbeddingRequest{
		Model:  "llama3",
		Prompt: "Why is the sky blue?",
	}
	c1.On("Embeddings", ctx, req).Once().Return(out1, nil)
	res, err = r.Embeddings(ctx, reqAlias)
	assert.NoError(t, err)
	assert.EqualValues(t, out1, res)

	req = &api.EmbeddingRequest{
		Model:  "other_model",
		Prompt: "Why is the sky blue?",
	}
	out2 := &api.EmbeddingResponse{
		Embedding: []float64{
			0.8785552978515625, -0.34576427936553955, 0.5742510557174683, -0.04222835972905159, -0.137906014919281,
			0.5670403838157654, 0.009260174818336964, 0.23178744316101074, -0.2916173040866852, -0.8924556970596313,
		},
	}
	c2.On("Embeddings", ctx, req).Once().Return(out2, nil)
	res, err = r.Embeddings(ctx, req)
	assert.NoError(t, err)
	assert.EqualValues(t, out2, res)

	reqAlias = &api.EmbeddingRequest{
		Model:  "some_alias",
		Prompt: "Why is the sky blue?",
	}
	c2.On("Embeddings", ctx, req).Once().Return(out2, nil)
	res, err = r.Embeddings(ctx, reqAlias)
	assert.NoError(t, err)
	assert.EqualValues(t, out2, res)

	req = &api.EmbeddingRequest{
		Model:  "other_model",
		Prompt: "Not a sensible request",
	}
	c2.On("Embeddings", ctx, req).Once().Return(nil, errors.New("some error"))
	res, err = r.Embeddings(ctx, req)
	assert.EqualError(t, err, "some error")
	assert.Nil(t, res)

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}

// Generate(ctx context.Context, req *api.GenerateRequest, fn api.GenerateResponseFunc) error
func TestRouterGenerate(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	}, gollamas.WithAlias("llama3", "llama3.2"), gollamas.WithAlias("some_alias", "other_model"))
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)
	cb := func(api.GenerateResponse) error { return nil }

	err = r.Generate(ctx, &api.GenerateRequest{
		Model: "unknown_model",
	}, cb)
	assert.EqualError(t, err, "gollamas router is missing a valid route to model unknown_model")

	req := &api.GenerateRequest{
		Model: "llama3.2",
	}
	c1.On("Generate", ctx, req, mock.Anything).Once().Return(nil)
	err = r.Generate(ctx, req, cb)
	assert.NoError(t, err)

	reqAlias := &api.GenerateRequest{
		Model: "llama3",
	}
	c1.On("Generate", ctx, req, mock.Anything).Once().Return(nil)
	err = r.Generate(ctx, reqAlias, cb)
	assert.NoError(t, err)

	req = &api.GenerateRequest{
		Model: "other_model",
	}
	c2.On("Generate", ctx, req, mock.Anything).Once().Return(nil)
	err = r.Generate(ctx, req, cb)
	assert.NoError(t, err)

	reqAlias = &api.GenerateRequest{
		Model: "some_alias",
	}
	c2.On("Generate", ctx, req, mock.Anything).Once().Return(nil)
	err = r.Generate(ctx, reqAlias, cb)
	assert.NoError(t, err)

	c2.On("Generate", ctx, req, mock.Anything).Once().Return(errors.New("some error"))
	err = r.Generate(ctx, req, cb)
	assert.EqualError(t, err, "some error")

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}

// Heartbeat(ctx context.Context) error
func TestRouterHeartbeat(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	}, gollamas.WithAlias("llama3", "llama3.2"), gollamas.WithAlias("some_alias", "other_model"))
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	c1.On("Heartbeat", ctx).Once().Return(nil)
	c2.On("Heartbeat", ctx).Once().Return(nil)
	err = r.Heartbeat(ctx)
	assert.NoError(t, err)

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}

// List(ctx context.Context) (*api.ListResponse, error)
func TestRouterListNoExposeAliases(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	}, gollamas.WithAlias("llama3", "llama3.2"), gollamas.WithAlias("some_alias", "other_model"), gollamas.WithExposeAliases(false))
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	c1.On("List", ctx).Once().Return(&api.ListResponse{Models: []api.ListModelResponse{{Model: "llama3.2:latest", Name: "llama3.2:latest", ModifiedAt: time.UnixMilli(123456789)}}}, nil)
	c2.On("List", ctx).Once().Return(&api.ListResponse{Models: []api.ListModelResponse{{Model: "other_model:latest", Name: "other_model:latest", ModifiedAt: time.UnixMilli(23456789)}}}, nil)

	out0 := &api.ListResponse{Models: []api.ListModelResponse{
		{Model: "llama3.2:latest", Name: "llama3.2:latest", ModifiedAt: time.UnixMilli(123456789)},
		{Model: "other_model:latest", Name: "other_model:latest", ModifiedAt: time.UnixMilli(23456789)},
	}}
	resp, err := r.List(ctx)
	assert.NoError(t, err)
	assert.EqualValues(t, out0, resp)

	c1.On("List", ctx).Once().Return(&api.ListResponse{Models: []api.ListModelResponse{{Model: "llama3.2", Name: "llama3.2", ModifiedAt: time.UnixMilli(123456789)}}}, nil)
	c2.On("List", ctx).Once().Return(&api.ListResponse{Models: []api.ListModelResponse{{Model: "other_model", Name: "other_model", ModifiedAt: time.UnixMilli(23456789)}}}, nil)

	out1 := &api.ListResponse{Models: []api.ListModelResponse{
		{Model: "llama3.2", Name: "llama3.2", ModifiedAt: time.UnixMilli(123456789)},
		{Model: "other_model", Name: "other_model", ModifiedAt: time.UnixMilli(23456789)},
	}}
	resp, err = r.List(ctx)
	assert.NoError(t, err)
	assert.EqualValues(t, out1, resp)

	c1.On("List", ctx).Once().Return(&api.ListResponse{Models: []api.ListModelResponse{{Model: "llama3.2", Name: "llama3.2"}}}, nil)
	c2.On("List", ctx).Once().Return(nil, errors.New("some error"))

	out2 := &api.ListResponse{Models: []api.ListModelResponse{
		{Model: "llama3.2", Name: "llama3.2"},
	}}
	resp, err = r.List(ctx)
	assert.NoError(t, err)
	assert.EqualValues(t, out2, resp)

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}

func TestRouterList(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	}, gollamas.WithAlias("llama3", "llama3.2"), gollamas.WithAlias("some_alias", "other_model"))
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	c1.On("List", ctx).Once().Return(&api.ListResponse{Models: []api.ListModelResponse{{Model: "llama3.2", Name: "llama3.2", ModifiedAt: time.UnixMilli(123456789)}}}, nil)
	c2.On("List", ctx).Once().Return(&api.ListResponse{Models: []api.ListModelResponse{{Model: "other_model", Name: "other_model", ModifiedAt: time.UnixMilli(23456789)}}}, nil)

	out1 := &api.ListResponse{Models: []api.ListModelResponse{
		{Model: "llama3.2", Name: "llama3.2", ModifiedAt: time.UnixMilli(123456789)},
		{Model: "llama3", Name: "llama3", ModifiedAt: time.UnixMilli(123456789)},
		{Model: "other_model", Name: "other_model", ModifiedAt: time.UnixMilli(23456789)},
		{Model: "some_alias", Name: "some_alias", ModifiedAt: time.UnixMilli(23456789)},
	}}
	resp, err := r.List(ctx)
	assert.NoError(t, err)
	assert.EqualValues(t, out1, resp)

	c1.On("List", ctx).Once().Return(&api.ListResponse{Models: []api.ListModelResponse{{Model: "llama3.2", Name: "llama3.2"}}}, nil)
	c2.On("List", ctx).Once().Return(nil, errors.New("some error"))

	out2 := &api.ListResponse{Models: []api.ListModelResponse{
		{Model: "llama3.2", Name: "llama3.2"},
		{Model: "llama3", Name: "llama3"},
	}}
	resp, err = r.List(ctx)
	assert.NoError(t, err)
	assert.EqualValues(t, out2, resp)

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}

// ListRunning(ctx context.Context) (*api.ProcessResponse, error)
func TestRouterListRunningNoExposeAliases(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	}, gollamas.WithAlias("llama3", "llama3.2"), gollamas.WithAlias("some_alias", "other_model"), gollamas.WithExposeAliases(false))
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	c1.On("ListRunning", ctx).Once().Return(&api.ProcessResponse{Models: []api.ProcessModelResponse{{Model: "llama3.2"}}}, nil)
	c2.On("ListRunning", ctx).Once().Return(&api.ProcessResponse{Models: []api.ProcessModelResponse{{Model: "other_model"}}}, nil)

	out1 := &api.ProcessResponse{Models: []api.ProcessModelResponse{
		{Model: "llama3.2"},
		{Model: "other_model"},
	}}
	resp, err := r.ListRunning(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.ElementsMatch(t, out1.Models, resp.Models)

	c1.On("ListRunning", ctx).Once().Return(&api.ProcessResponse{Models: []api.ProcessModelResponse{{Model: "llama3.2"}}}, nil)
	c2.On("ListRunning", ctx).Once().Return(nil, errors.New("some error"))

	out2 := &api.ProcessResponse{Models: []api.ProcessModelResponse{{Model: "llama3.2"}}}
	resp, err = r.ListRunning(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.ElementsMatch(t, out2.Models, resp.Models)

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}

func TestRouterListRunning(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	}, gollamas.WithAlias("llama3", "llama3.2"), gollamas.WithAlias("some_alias", "other_model"))
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	c1.On("ListRunning", ctx).Once().Return(&api.ProcessResponse{Models: []api.ProcessModelResponse{{Model: "llama3.2", Name: "llama3.2"}}}, nil)
	c2.On("ListRunning", ctx).Once().Return(&api.ProcessResponse{Models: []api.ProcessModelResponse{{Model: "other_model", Name: "other_model"}}}, nil)

	out1 := &api.ProcessResponse{Models: []api.ProcessModelResponse{
		{Model: "llama3.2", Name: "llama3.2"},
		{Model: "llama3", Name: "llama3"},
		{Model: "other_model", Name: "other_model"},
		{Model: "some_alias", Name: "some_alias"},
	}}
	resp, err := r.ListRunning(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.ElementsMatch(t, out1.Models, resp.Models)

	c1.On("ListRunning", ctx).Once().Return(&api.ProcessResponse{Models: []api.ProcessModelResponse{{Model: "llama3.2", Name: "llama3.2"}}}, nil)
	c2.On("ListRunning", ctx).Once().Return(nil, errors.New("some error"))

	out2 := &api.ProcessResponse{Models: []api.ProcessModelResponse{
		{Model: "llama3.2", Name: "llama3.2"},
		{Model: "llama3", Name: "llama3"},
	}}
	resp, err = r.ListRunning(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.ElementsMatch(t, out2.Models, resp.Models)

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}

// Pull(ctx context.Context, req *api.PullRequest, fn api.PullProgressFunc) error
func TestRouterPull(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	}, gollamas.WithAlias("llama3", "llama3.2"), gollamas.WithAlias("some_alias", "other_model"))
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)
	cb := func(api.ProgressResponse) error { return nil }

	err = r.Pull(ctx, &api.PullRequest{
		Model: "unknown_model",
	}, cb)
	assert.EqualError(t, err, "gollamas router is missing a valid route to model unknown_model")

	req := &api.PullRequest{
		Model: "llama3.2",
	}
	c1.On("Pull", ctx, req, mock.Anything).Once().Return(nil)
	err = r.Pull(ctx, req, cb)
	assert.NoError(t, err)

	reqAlias := &api.PullRequest{
		Model: "llama3",
	}
	c1.On("Pull", ctx, req, mock.Anything).Once().Return(nil)
	err = r.Pull(ctx, reqAlias, cb)
	assert.NoError(t, err)

	req = &api.PullRequest{
		Name: "other_model",
	}
	c2.On("Pull", ctx, req, mock.Anything).Once().Return(nil)
	err = r.Pull(ctx, req, cb)
	assert.NoError(t, err)

	reqAlias = &api.PullRequest{
		Name: "some_alias",
	}
	c2.On("Pull", ctx, req, mock.Anything).Once().Return(nil)
	err = r.Pull(ctx, reqAlias, cb)
	assert.NoError(t, err)

	c2.On("Pull", ctx, req, mock.Anything).Once().Return(errors.New("some error"))
	err = r.Pull(ctx, req, cb)
	assert.EqualError(t, err, "some error")

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}

// Push(ctx context.Context, req *api.PushRequest, fn api.PushProgressFunc) error
func TestRouterPush(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	})
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	cb := func(api.ProgressResponse) error { return nil }

	req := &api.PushRequest{
		Model: "llama3.2",
	}
	err = r.Push(ctx, req, cb)
	assert.EqualError(t, err, "gollamas: router doesn't support pushing models")

	req = &api.PushRequest{
		Model: "other_model",
	}
	err = r.Push(ctx, req, cb)
	assert.EqualError(t, err, "gollamas: router doesn't support pushing models")

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}

// Show(ctx context.Context, req *api.ShowRequest) (*api.ShowResponse, error)
func TestRouterShow(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	}, gollamas.WithAlias("llama3", "llama3.2"), gollamas.WithAlias("some_alias", "other_model"))
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	resp, err := r.Show(ctx, &api.ShowRequest{
		Model: "unknown_model",
	})
	assert.EqualError(t, err, "gollamas router is missing a valid route to model unknown_model")
	assert.Nil(t, resp)

	req := &api.ShowRequest{
		Model: "llama3.2",
	}
	out1 := &api.ShowResponse{
		Modelfile: "llama3.2.file",
	}
	c1.On("Show", ctx, req, mock.Anything).Once().Return(out1, nil)
	resp, err = r.Show(ctx, req)
	assert.NoError(t, err)
	assert.EqualValues(t, out1, resp)

	reqAlias := &api.ShowRequest{
		Model: "llama3",
	}
	c1.On("Show", ctx, req, mock.Anything).Once().Return(out1, nil)
	resp, err = r.Show(ctx, reqAlias)
	assert.NoError(t, err)
	assert.EqualValues(t, out1, resp)

	req = &api.ShowRequest{
		Name: "other_model",
	}
	out2 := &api.ShowResponse{
		Modelfile: "other_model.file",
	}
	c2.On("Show", ctx, req, mock.Anything).Once().Return(out2, nil)
	resp, err = r.Show(ctx, req)
	assert.NoError(t, err)
	assert.EqualValues(t, out2, resp)

	reqAlias = &api.ShowRequest{
		Name: "some_alias",
	}
	c2.On("Show", ctx, req, mock.Anything).Once().Return(out2, nil)
	resp, err = r.Show(ctx, reqAlias)
	assert.NoError(t, err)
	assert.EqualValues(t, out2, resp)

	c2.On("Show", ctx, req, mock.Anything).Once().Return(nil, errors.New("some error"))
	resp, err = r.Show(ctx, req)
	assert.EqualError(t, err, "some error")
	assert.Nil(t, resp)

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}

// Version(ctx context.Context) (string, error)
func TestRouterVersion(t *testing.T) {
	c1 := mocks.NewIOllamaClient(t)
	c2 := mocks.NewIOllamaClient(t)
	ctx, cancel, r, err := newRouter(map[string]gollamas.IOllamaClient{
		"llama3.2":    c1,
		"other_model": c2,
	}, gollamas.WithAlias("llama3", "llama3.2"), gollamas.WithAlias("some_alias", "other_model"))
	defer cancel()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	c1.On("Version", ctx).Once().Return("0.5.1", nil)
	c2.On("Version", ctx).Once().Return("0.5.2", nil)
	resp, err := r.Version(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "0.5.1", resp)

	c1.AssertExpectations(t)
	c2.AssertExpectations(t)
}
