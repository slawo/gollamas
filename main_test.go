package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ollama/ollama/api"
	gollamas "github.com/slawo/gollamas"
	"github.com/slawo/gollamas/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestInitServiceFailOnMissingProxyConfig(t *testing.T) {
	_, err := gollamas.InitService(gollamas.GollamasConfig{
		Connections: map[gollamas.ConnectionID]gollamas.ConnectionConfig{gollamas.ConnectionID("conn1"): {Url: "http://localhost:8080"}},
	})
	assert.EqualError(t, err, "missing models config")
}

func TestInitServiceFailOnEmptyProxyConfig(t *testing.T) {
	s, err := gollamas.InitService(gollamas.GollamasConfig{
		Models:      map[gollamas.ModelID]gollamas.ModelConfig{},
		Connections: map[gollamas.ConnectionID]gollamas.ConnectionConfig{gollamas.ConnectionID("conn1"): {Url: "http://localhost:8080"}},
	})
	assert.EqualError(t, err, "empty models config")
	assert.Nil(t, s)
}

func TestInitServiceFailOnMissingConnectionsConfig(t *testing.T) {
	s, err := gollamas.InitService(gollamas.GollamasConfig{
		Models: map[gollamas.ModelID]gollamas.ModelConfig{"model1": {ConnectionID: "conn1"}},
	})
	assert.EqualError(t, err, "missing connections config")
	assert.Nil(t, s)
}

func TestInitServiceSucceedsOnEmptyConnectionsConfig(t *testing.T) {
	s, err := gollamas.InitService(gollamas.GollamasConfig{
		Connections: map[gollamas.ConnectionID]gollamas.ConnectionConfig{},
		Models:      map[gollamas.ModelID]gollamas.ModelConfig{"model1": {ConnectionID: "http://localhost:8080"}},
	})
	assert.NoError(t, err)
	assert.NotNil(t, s)
}

func TestInitServiceFailsOnUnmetConnectionIdConfig(t *testing.T) {
	s, err := gollamas.InitService(gollamas.GollamasConfig{
		Connections: map[gollamas.ConnectionID]gollamas.ConnectionConfig{},
		Models:      map[gollamas.ModelID]gollamas.ModelConfig{"model1": {ConnectionID: "unknown"}},
	})
	assert.EqualError(t, err, "invalid connection id: unknown, invalid url scheme")
	assert.Nil(t, s)
}

func TestInitServiceFailOnFailedAliases(t *testing.T) {
	s, err := gollamas.InitService(gollamas.GollamasConfig{
		Connections: map[gollamas.ConnectionID]gollamas.ConnectionConfig{"conn1": {Url: "http://localhost:8080"}},
		Models:      map[gollamas.ModelID]gollamas.ModelConfig{"model1": {ConnectionID: "conn1"}},
		Aliases:     map[gollamas.ModelID]gollamas.ModelID{"alias1": "unknown_model"},
	})
	assert.EqualError(t, err, "alias alias1 points to unknown model unknown_model")
	assert.Nil(t, s)
}

func TestInitService(t *testing.T) {
	s, err := gollamas.InitService(gollamas.GollamasConfig{
		Connections: map[gollamas.ConnectionID]gollamas.ConnectionConfig{"http://localhost:8080": {Url: "http://localhost:8080"}},
		Models:      map[gollamas.ModelID]gollamas.ModelConfig{"model1": {ConnectionID: "http://localhost:8080"}},
	})
	assert.NoError(t, err)
	assert.NotNil(t, s)
	s, err = gollamas.InitService(gollamas.GollamasConfig{
		Connections: map[gollamas.ConnectionID]gollamas.ConnectionConfig{"conn1": {Url: "http://localhost:8080"}},
		Models:      map[gollamas.ModelID]gollamas.ModelConfig{"model1": {ConnectionID: "conn1"}},
	})
	assert.NoError(t, err)
	assert.NotNil(t, s)
	s, err = gollamas.InitService(gollamas.GollamasConfig{
		Connections: map[gollamas.ConnectionID]gollamas.ConnectionConfig{"conn1": {Url: "http://localhost:8080"}},
		Models:      map[gollamas.ModelID]gollamas.ModelConfig{"model1": {ConnectionID: "conn1"}},
		Aliases:     map[gollamas.ModelID]gollamas.ModelID{"alias1": "model1"},
	})
	assert.NoError(t, err)
	assert.NotNil(t, s)
}

func TestRunGollamas(t *testing.T) {
	type test struct {
		method       string
		url          string
		request      string
		expectedcode int
		expected     any
		prep         func(t *testing.T, mcs map[string]*mocks.IGinService)
		conf         gollamas.GollamasConfig
	}
	tests := map[string]test{
		"chat1": {
			method:       "POST",
			url:          "/api/chat",
			request:      `{"messages":[{"role":"user","content":"hello"}], "model":"model1"}`,
			expectedcode: 200,
			expected:     `{"created_at":"0001-01-01T00:00:00Z", "done":true, "message":{"content":"hello user", "role":"ai"}, "model":"model1"}`,
			prep: func(t *testing.T, mcs map[string]*mocks.IGinService) {
				mcs["model1"].On("ChatHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRequestResponseInGinContext(t, func(req *api.ChatRequest) *api.ChatResponse {
					assert.EqualValues(t, &api.ChatRequest{
						Model:    "model1",
						Messages: []api.Message{{Role: "user", Content: "hello"}},
					}, req)
					return &api.ChatResponse{Message: api.Message{Role: "ai", Content: "hello user"}, Model: req.Model, Done: true}
				}))
			},
		},
		"chat2": {
			method:       "POST",
			url:          "/api/chat",
			request:      `{"messages":[{"role":"user","content":"hello"}], "model":"model2:4b"}`,
			expectedcode: 200,
			expected:     &api.ChatResponse{Message: api.Message{Role: "ai", Content: "hello human"}, Model: "model2:4b", Done: true},
			prep: func(t *testing.T, mcs map[string]*mocks.IGinService) {
				mcs["model2:4b"].On("ChatHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRequestResponseInGinContext(t, func(req *api.ChatRequest) *api.ChatResponse {
					assert.EqualValues(t, &api.ChatRequest{
						Model:    "model2:4b",
						Messages: []api.Message{{Role: "user", Content: "hello"}},
					}, req)
					return &api.ChatResponse{Message: api.Message{Role: "ai", Content: "hello human"}, Model: req.Model, Done: true}
				}))
			},
		},
		"copy": {
			method:       "POST",
			url:          "/api/copy",
			request:      `{ "source": "model1", "destination": "model1-backup"}`,
			expectedcode: 404,
			expected:     `{"error":"gollamas router doesn't support copying models"}`,
		},
		"create": {
			method:       "POST",
			url:          "/api/create",
			request:      `{ "source": "model1", "destination": "model1-backup"}`,
			expectedcode: 404,
			expected:     `{"error":"gollamas router doesn't support creating models"}`,
		},
		"create blob": {
			method:       "POST",
			url:          "/api/blobs/some_digest",
			expectedcode: 404,
			expected:     `{"error":"gollamas router doesn't like blobs (not supported)"}`,
		},
		"delete": {
			method:       "DELETE",
			url:          "/api/delete",
			expectedcode: 404,
			expected:     `{"error":"gollamas router doesn't support deleting models"}`,
		},
		"embedings/model1": {
			method:       "POST",
			url:          "/api/embeddings",
			request:      `{ "model": "model1", "prompt": "Here is an article about llamas..." }`,
			expectedcode: 200,
			expected:     `{"embedding":[]}`,
			prep: func(t *testing.T, mcs map[string]*mocks.IGinService) {
				mcs["model1"].On("EmbeddingsHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRequestResponseInGinContext(t, func(req *api.EmbeddingRequest) *api.EmbeddingResponse {
					assert.EqualValues(t, &api.EmbeddingRequest{
						Model:  "model1",
						Prompt: "Here is an article about llamas...",
					}, req)
					return &api.EmbeddingResponse{Embedding: []float64{}}
				}))
			},
		},
		"embedings/model2:4b": {
			method:       "POST",
			url:          "/api/embeddings",
			request:      `{ "model": "model2:4b", "prompt": "Here is an article about llamas..." }`,
			expectedcode: 200,
			expected:     `{"embedding":[]}`,
			prep: func(t *testing.T, mcs map[string]*mocks.IGinService) {
				mcs["model2:4b"].On("EmbeddingsHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRequestResponseInGinContext(t, func(req *api.EmbeddingRequest) *api.EmbeddingResponse {
					assert.EqualValues(t, &api.EmbeddingRequest{
						Model:  "model2:4b",
						Prompt: "Here is an article about llamas...",
					}, req)
					return &api.EmbeddingResponse{Embedding: []float64{}}
				}))
			},
		},
		"embedings/unknown": {
			method:       "POST",
			url:          "/api/embeddings",
			request:      `{ "model": "unknown", "prompt": "Here is an article about llamas..." }`,
			expectedcode: 404,
			expected:     `{"error":"gollamas router is missing a valid route to model unknown"}`,
		},

		"tags(no aliases)": {
			method:       "GET",
			url:          "/api/tags",
			expectedcode: 200,
			expected: api.ListResponse{Models: []api.ListModelResponse{
				{Model: "model2:4b", Name: "model2:4b", ModifiedAt: time.UnixMilli(1741600020000)},
				{Model: "model1", Name: "model1", ModifiedAt: time.UnixMilli(1741600010000)},
			}},
			prep: func(t *testing.T, mcs map[string]*mocks.IGinService) {
				mcs["model1"].On("ListHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(c *gin.Context) {
					c.JSON(http.StatusOK, api.ListResponse{Models: []api.ListModelResponse{
						{Model: "model1", Name: "model1", ModifiedAt: time.UnixMilli(1741600010000)},
						{Model: "other:2b", Name: "other:2b", ModifiedAt: time.UnixMilli(1741600050000)},
					}})
				}))
				mcs["model2:4b"].On("ListHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(c *gin.Context) {
					c.JSON(http.StatusOK, api.ListResponse{Models: []api.ListModelResponse{
						{Model: "model2:4b", Name: "model2:4b", ModifiedAt: time.UnixMilli(1741600020000)},
						{Model: "other:2b", Name: "other:2b", ModifiedAt: time.UnixMilli(1741600050000)},
					}})
				}))
			},
		},
		"list(with aliases)": {
			method:       "GET",
			url:          "/api/tags",
			expectedcode: 200,
			conf: gollamas.GollamasConfig{
				Aliases: map[gollamas.ModelID]gollamas.ModelID{"alias1": "model1"},
			},
			expected: api.ListResponse{Models: []api.ListModelResponse{
				{Model: "model2:4b", Name: "model2:4b", ModifiedAt: time.UnixMilli(1741600020000)},
				{Model: "model1", Name: "model1", ModifiedAt: time.UnixMilli(1741600010000)},
			}},
			prep: func(t *testing.T, mcs map[string]*mocks.IGinService) {
				mcs["model1"].On("ListHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(c *gin.Context) {
					c.JSON(http.StatusOK, api.ListResponse{Models: []api.ListModelResponse{
						{Model: "model1", Name: "model1", ModifiedAt: time.UnixMilli(1741600010000)},
						{Model: "other:2b", Name: "other:2b", ModifiedAt: time.UnixMilli(1741600050000)},
					}})
				}))
				mcs["model2:4b"].On("ListHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(c *gin.Context) {
					c.JSON(http.StatusOK, api.ListResponse{Models: []api.ListModelResponse{
						{Model: "model2:4b", Name: "model2:4b", ModifiedAt: time.UnixMilli(1741600020000)},
						{Model: "other:2b", Name: "other:2b", ModifiedAt: time.UnixMilli(1741600050000)},
					}})
				}))
			},
		},
		"list(with aliases and list aliases)": {
			method:       "GET",
			url:          "/api/tags",
			expectedcode: 200,
			conf: gollamas.GollamasConfig{
				Aliases:     map[gollamas.ModelID]gollamas.ModelID{"alias1": "model1"},
				ListAliases: true,
			},
			expected: api.ListResponse{Models: []api.ListModelResponse{
				{Model: "model2:4b", Name: "model2:4b", ModifiedAt: time.UnixMilli(1741600020000)},
				{Model: "model1", Name: "model1", ModifiedAt: time.UnixMilli(1741600010000)},
				{Model: "alias1", Name: "alias1", ModifiedAt: time.UnixMilli(1741600010000)},
			}},
			prep: func(t *testing.T, mcs map[string]*mocks.IGinService) {
				mcs["model1"].On("ListHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(c *gin.Context) {
					c.JSON(http.StatusOK, api.ListResponse{Models: []api.ListModelResponse{
						{Model: "model1", Name: "model1", ModifiedAt: time.UnixMilli(1741600010000)},
						{Model: "other:2b", Name: "other:2b", ModifiedAt: time.UnixMilli(1741600050000)},
					}})
				}))
				mcs["model2:4b"].On("ListHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(c *gin.Context) {
					c.JSON(http.StatusOK, api.ListResponse{Models: []api.ListModelResponse{
						{Model: "model2:4b", Name: "model2:4b", ModifiedAt: time.UnixMilli(1741600020000)},
						{Model: "other:2b", Name: "other:2b", ModifiedAt: time.UnixMilli(1741600050000)},
					}})
				}))
			},
		},
		"list(filter out cross server models)": {
			method:       "GET",
			url:          "/api/tags",
			expectedcode: 200,
			conf: gollamas.GollamasConfig{
				Aliases:     map[gollamas.ModelID]gollamas.ModelID{"alias1": "model1"},
				ListAliases: true,
			},
			expected: api.ListResponse{Models: []api.ListModelResponse{
				{Model: "model1", Name: "model1", ModifiedAt: time.UnixMilli(1741600010000)},
				{Model: "alias1", Name: "alias1", ModifiedAt: time.UnixMilli(1741600010000)},
			}},
			prep: func(t *testing.T, mcs map[string]*mocks.IGinService) {
				mcs["model1"].On("ListHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(c *gin.Context) {
					c.JSON(http.StatusOK, api.ListResponse{Models: []api.ListModelResponse{
						{Model: "model1", Name: "model1", ModifiedAt: time.UnixMilli(1741600010000)},
						{Model: "model2:4b", Name: "model2:4b", ModifiedAt: time.UnixMilli(1741600020000)},
						{Model: "other:2b", Name: "other:2b", ModifiedAt: time.UnixMilli(1741600050000)},
					}})
				}))
				mcs["model2:4b"].On("ListHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(c *gin.Context) {
					c.JSON(http.StatusOK, api.ListResponse{Models: []api.ListModelResponse{
						// {Model: "model2:4b", Name: "model2:4b", ModifiedAt: time.UnixMilli(1741600020000)},
						// {Model: "other:2b", Name: "other:2b", ModifiedAt: time.UnixMilli(1741600050000)},
					}})
				}))
			},
		},
		"ps(with aliases)": {
			method:       "GET",
			url:          "/api/ps",
			expectedcode: 200,
			expected: api.ProcessResponse{Models: []api.ProcessModelResponse{
				{Model: "model1", Name: "model1", ExpiresAt: time.UnixMilli(1741600010000)},
				{Model: "model2:4b", Name: "model2:4b", ExpiresAt: time.UnixMilli(1741600020000)},
			}},
			prep: func(t *testing.T, mcs map[string]*mocks.IGinService) {
				mcs["model1"].On("PsHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(c *gin.Context) {
					c.JSON(http.StatusOK, api.ProcessResponse{Models: []api.ProcessModelResponse{
						{Model: "model1", Name: "model1", ExpiresAt: time.UnixMilli(1741600010000)},
						{Model: "other:2b", Name: "other:2b", ExpiresAt: time.UnixMilli(1741600050000)},
					}})
				}))
				mcs["model2:4b"].On("PsHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(c *gin.Context) {
					c.JSON(http.StatusOK, api.ProcessResponse{Models: []api.ProcessModelResponse{
						{Model: "model2:4b", Name: "model2:4b", ExpiresAt: time.UnixMilli(1741600020000)},
						{Model: "other:2b", Name: "other:2b", ExpiresAt: time.UnixMilli(1741600050000)},
					}})
				}))
			},
		},
		"ps()": {
			method:       "GET",
			url:          "/api/ps",
			expectedcode: 200,
			conf: gollamas.GollamasConfig{
				Aliases: map[gollamas.ModelID]gollamas.ModelID{"alias1": "model1"},
			},
			expected: api.ProcessResponse{Models: []api.ProcessModelResponse{
				{Model: "model1", Name: "model1", ExpiresAt: time.UnixMilli(1741600010000)},
				{Model: "model2:4b", Name: "model2:4b", ExpiresAt: time.UnixMilli(1741600020000)},
			}},
			prep: func(t *testing.T, mcs map[string]*mocks.IGinService) {
				mcs["model1"].On("PsHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(c *gin.Context) {
					c.JSON(http.StatusOK, api.ProcessResponse{Models: []api.ProcessModelResponse{
						{Model: "model1", Name: "model1", ExpiresAt: time.UnixMilli(1741600010000)},
						{Model: "other:2b", Name: "other:2b", ExpiresAt: time.UnixMilli(1741600050000)},
					}})
				}))
				mcs["model2:4b"].On("PsHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(c *gin.Context) {
					c.JSON(http.StatusOK, api.ProcessResponse{Models: []api.ProcessModelResponse{
						{Model: "model2:4b", Name: "model2:4b", ExpiresAt: time.UnixMilli(1741600020000)},
						{Model: "other:2b", Name: "other:2b", ExpiresAt: time.UnixMilli(1741600050000)},
					}})
				}))
			},
		},
		"ps(with aliases and list aliases)": {
			method:       "GET",
			url:          "/api/ps",
			expectedcode: 200,
			conf: gollamas.GollamasConfig{
				Aliases:     map[gollamas.ModelID]gollamas.ModelID{"alias1": "model1"},
				ListAliases: true,
			},
			expected: api.ProcessResponse{Models: []api.ProcessModelResponse{
				{Model: "alias1", Name: "alias1", ExpiresAt: time.UnixMilli(1741600010000)},
				{Model: "model1", Name: "model1", ExpiresAt: time.UnixMilli(1741600010000)},
				{Model: "model2:4b", Name: "model2:4b", ExpiresAt: time.UnixMilli(1741600020000)},
			}},
			prep: func(t *testing.T, mcs map[string]*mocks.IGinService) {
				mcs["model1"].On("PsHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(c *gin.Context) {
					c.JSON(http.StatusOK, api.ProcessResponse{Models: []api.ProcessModelResponse{
						{Model: "model1", Name: "model1", ExpiresAt: time.UnixMilli(1741600010000)},
						{Model: "other:2b", Name: "other:2b", ExpiresAt: time.UnixMilli(1741600050000)},
					}})
				}))
				mcs["model2:4b"].On("PsHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(c *gin.Context) {
					c.JSON(http.StatusOK, api.ProcessResponse{Models: []api.ProcessModelResponse{
						{Model: "model2:4b", Name: "model2:4b", ExpiresAt: time.UnixMilli(1741600020000)},
						{Model: "other:2b", Name: "other:2b", ExpiresAt: time.UnixMilli(1741600050000)},
					}})
				}))
			},
		},
		"ps(filter out cross server models)": {
			method:       "GET",
			url:          "/api/ps",
			expectedcode: 200,
			conf: gollamas.GollamasConfig{
				Aliases:     map[gollamas.ModelID]gollamas.ModelID{"alias1": "model1"},
				ListAliases: true,
			},
			expected: api.ProcessResponse{Models: []api.ProcessModelResponse{
				{Model: "alias1", Name: "alias1", ExpiresAt: time.UnixMilli(1741600010000)},
				{Model: "model1", Name: "model1", ExpiresAt: time.UnixMilli(1741600010000)},
			}},
			prep: func(t *testing.T, mcs map[string]*mocks.IGinService) {
				mcs["model1"].On("PsHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(c *gin.Context) {
					c.JSON(http.StatusOK, api.ProcessResponse{Models: []api.ProcessModelResponse{
						{Model: "model1", Name: "model1", ExpiresAt: time.UnixMilli(1741600010000)},
						{Model: "model2:4b", Name: "model2:4b", ExpiresAt: time.UnixMilli(1741600020000)},
						{Model: "other:2b", Name: "other:2b", ExpiresAt: time.UnixMilli(1741600050000)},
					}})
				}))
				mcs["model2:4b"].On("PsHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(c *gin.Context) {
					c.JSON(http.StatusOK, api.ProcessResponse{Models: []api.ProcessModelResponse{
						// {Model: "model2:4b", Name: "model2:4b", ModifiedAt: time.UnixMilli(1741600020000)},
						// {Model: "other:2b", Name: "other:2b", ModifiedAt: time.UnixMilli(1741600050000)},
					}})
				}))
			},
		},

		// EmbeddingsHandler(c *gin.Context)
		// EmbedHandler(c *gin.Context)
		// GenerateHandler(c *gin.Context)
		// HeadBlobHandler(c *gin.Context)
		// HomeHandler(c *gin.Context)
		// ListHandler(c *gin.Context)
		// PullHandler(c *gin.Context)
		// PushHandler(c *gin.Context)
		// ShowHandler(c *gin.Context)
		// VersionHandler(c *gin.Context)
	}

	for name, test := range tests {
		tt := test
		t.Run(name, func(t *testing.T) {
			runAutoConfig(t, context.Background(), tt.conf, func(
				ctx context.Context,
				mcs map[string]*mocks.IGinService,
				sr http.Handler,
			) {
				if tt.prep != nil {
					tt.prep(t, mcs)
				}
				hreq, _ := http.NewRequest(tt.method, tt.url, bytes.NewBuffer([]byte(tt.request)))
				w := CreateTestResponseRecorder()
				sr.ServeHTTP(w, hreq)
				assert.Equal(t, tt.expectedcode, w.Code)

				var str string
				switch r := tt.expected.(type) {
				case string:
					str = r
				default:
					dt, err := json.Marshal(tt.expected)
					assert.NoError(t, err)
					str = string(dt)
				}

				if json.Unmarshal([]byte(str), &map[string]any{}) == nil && json.Unmarshal(w.Body.Bytes(), &map[string]any{}) == nil {
					assert.JSONEq(t, str, w.Body.String())
				} else {
					assert.Equal(t, str, w.Body.String())
				}
				for _, m := range mcs {
					m.AssertExpectations(t)
				}
			})
		})
	}
}

func createListener() (l net.Listener, close func()) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	return l, func() {
		_ = l.Close()
	}
}

func runWithinGinMocksBounds(ctx context.Context, svcs map[gollamas.ConnectionID]*mocks.IGinService, fn func(map[gollamas.ConnectionID]gollamas.ConnectionConfig)) {
	cfg := map[gollamas.ConnectionID]gollamas.ConnectionConfig{}
	for id, svc := range svcs {
		l, close := createListener()
		defer close()
		cfg[id] = gollamas.ConnectionConfig{Url: "http://" + l.Addr().String(), ConnectionID: id}
		rs := gollamas.GenerateRoutes(svc)
		go func(l net.Listener, rs *gin.Engine) {
			_ = http.Serve(l, rs)
		}(l, rs)
	}
	fn(cfg)
}

func runAutoConfig(t *testing.T, ctx context.Context, cfg gollamas.GollamasConfig, fn func(context.Context, map[string]*mocks.IGinService, http.Handler)) {
	mcs := map[string]*mocks.IGinService{}
	tcs := map[gollamas.ConnectionID]*mocks.IGinService{}

	if len(cfg.Models) == 0 {
		cfg.Models = map[gollamas.ModelID]gollamas.ModelConfig{}
		for i, m := range []string{"model1", "model2:4b"} {
			cfg.Models[gollamas.ModelID(m)] = gollamas.ModelConfig{ConnectionID: gollamas.ConnectionID(fmt.Sprintf("c%d", i+1))}
		}
	}
	for m, c := range cfg.Models {
		if _, ok := tcs[c.ConnectionID]; !ok {
			tcs[c.ConnectionID] = mocks.NewIGinService(t)
		}
		mcs[m.String()] = tcs[c.ConnectionID]
	}

	runWithinGinMocksBounds(ctx, tcs, func(ccfg map[gollamas.ConnectionID]gollamas.ConnectionConfig) {
		c := gollamas.GollamasConfig{
			Listen:      "localhost:0",
			Connections: ccfg,
			Models:      cfg.Models,
			Aliases:     cfg.Aliases,
			ListAliases: cfg.ListAliases,
		}
		s, err := gollamas.InitService(c)
		assert.NoError(t, err)
		sr := gollamas.GenerateRoutes(s)
		fn(ctx, mcs, sr)
	})
}

func getRequestResponseInGinContext[I any, O any](t *testing.T, fn func(*I) *O) func(args mock.Arguments) {
	return func(args mock.Arguments) {
		c := args.Get(0).(*gin.Context)
		var req I
		gollamas.BindRequest(c, &req)
		resp := fn(&req)
		c.JSON(http.StatusOK, resp)
	}
}

func getRunInGinContext(t *testing.T, fn func(*gin.Context)) func(args mock.Arguments) {
	return func(args mock.Arguments) {
		c := args.Get(0).(*gin.Context)
		fn(c)
	}
}
