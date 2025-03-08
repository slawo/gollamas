package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ollama/ollama/api"
	gollamas "github.com/slawo/gollamas"
	"github.com/slawo/gollamas/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestInitServiceFailOnMissingProxyConfig(t *testing.T) {
	_, err := gollamas.InitService(gollamas.GollamasConfig{})
	assert.EqualError(t, err, "missing proxy config")
}

func TestInitServiceFailOnEmptyProxyConfig(t *testing.T) {
	_, err := gollamas.InitService(gollamas.GollamasConfig{
		Proxies: map[string]gollamas.ProxyConfig{},
	})
	assert.EqualError(t, err, "empty proxy config map")
}

func TestInitServiceFailOnFailedAliases(t *testing.T) {
	_, err := gollamas.InitService(gollamas.GollamasConfig{
		Proxies: map[string]gollamas.ProxyConfig{"model1": {Url: "http://localhost:8080"}},
		Aliases: map[string]string{"alias1": "unknown_model"},
	})
	assert.EqualError(t, err, "alias alias1 points to unknown model unknown_model")
}

func TestInitService(t *testing.T) {
	_, err := gollamas.InitService(gollamas.GollamasConfig{
		Proxies: map[string]gollamas.ProxyConfig{"model1": {Url: "http://localhost:8080"}},
	})
	assert.NoError(t, err)
	_, err = gollamas.InitService(gollamas.GollamasConfig{
		Proxies: map[string]gollamas.ProxyConfig{"model1": {Url: "http://localhost:8080"}},
		Aliases: map[string]string{"alias1": "model1"},
	})
	assert.NoError(t, err)
}

func TestRunGollamas(t *testing.T) {
	runAutoConfig(t, context.Background(), func(
		ctx context.Context,
		mcs map[string]*mocks.IGinService,
		sr http.Handler,
	) {
		type test struct {
			method       string
			url          string
			request      string
			expectedcode int
			expected     string
			prep         func(t *testing.T)
		}
		tests := map[string]test{
			"chat1": {
				method:       "POST",
				url:          "/api/chat",
				request:      `{"messages":[{"role":"user","content":"hello"}], "model":"model1"}`,
				expectedcode: 200,
				expected:     `{"created_at":"0001-01-01T00:00:00Z", "done":true, "message":{"content":"hello user", "role":"ai"}, "model":"model1"}`,
				prep: func(t *testing.T) {
					mcs["model1"].On("ChatHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(req *api.ChatRequest) *api.ChatResponse {
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
				expected:     `{"created_at":"0001-01-01T00:00:00Z", "done":true, "message":{"content":"hello human", "role":"ai"}, "model":"model2:4b"}`,
				prep: func(t *testing.T) {
					mcs["model2:4b"].On("ChatHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(req *api.ChatRequest) *api.ChatResponse {
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
				prep: func(t *testing.T) {
					mcs["model1"].On("EmbeddingsHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(req *api.EmbeddingRequest) *api.EmbeddingResponse {
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
				prep: func(t *testing.T) {
					mcs["model2:4b"].On("EmbeddingsHandler", mock.AnythingOfType("*gin.Context")).Once().Run(getRunInGinContext(t, func(req *api.EmbeddingRequest) *api.EmbeddingResponse {
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

			// EmbeddingsHandler(c *gin.Context)
			// EmbedHandler(c *gin.Context)
			// GenerateHandler(c *gin.Context)
			// HeadBlobHandler(c *gin.Context)
			// HomeHandler(c *gin.Context)
			// ListHandler(c *gin.Context)
			// PsHandler(c *gin.Context)
			// PullHandler(c *gin.Context)
			// PushHandler(c *gin.Context)
			// ShowHandler(c *gin.Context)
			// VersionHandler(c *gin.Context)
		}

		for name, test := range tests {
			tt := test
			t.Run(name, func(t *testing.T) {
				if tt.prep != nil {
					tt.prep(t)
				}
				hreq, _ := http.NewRequest(tt.method, tt.url, bytes.NewBuffer([]byte(tt.request)))
				w := CreateTestResponseRecorder()
				sr.ServeHTTP(w, hreq)
				assert.Equal(t, tt.expectedcode, w.Code)
				if json.Unmarshal([]byte(tt.expected), &map[string]any{}) == nil && json.Unmarshal(w.Body.Bytes(), &map[string]any{}) == nil {
					assert.JSONEq(t, tt.expected, w.Body.String())
				} else {
					assert.Equal(t, tt.expected, w.Body.String())
				}
				for _, m := range mcs {
					m.AssertExpectations(t)
				}
			})
		}
	})
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

func initServer() {
	l, close := createListener()
	defer close()
	http.Handle("/", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// handle like normal
	}))
	log.Println("listening at", l.Addr().(*net.TCPAddr).Port)
	http.Serve(l, nil)
}

func runWithinGinMocksBounds(ctx context.Context, svcs map[string]*mocks.IGinService, fn func(map[string]gollamas.ProxyConfig)) {
	cfg := map[string]gollamas.ProxyConfig{}
	for model, svc := range svcs {
		l, close := createListener()
		defer close()
		cfg[model] = gollamas.ProxyConfig{Url: "http://" + l.Addr().String()}
		rs := gollamas.GenerateRoutes(svc)
		go func(l net.Listener, rs *gin.Engine) {
			_ = http.Serve(l, rs)
		}(l, rs)
	}
	fn(cfg)
}

func runAutoConfig(t *testing.T, ctx context.Context, fn func(context.Context, map[string]*mocks.IGinService, http.Handler)) {
	mcs := map[string]*mocks.IGinService{
		"model1":    mocks.NewIGinService(t),
		"model2:4b": mocks.NewIGinService(t),
	}

	runWithinGinMocksBounds(ctx, mcs, func(m map[string]gollamas.ProxyConfig) {
		cfg := gollamas.GollamasConfig{
			Listen:  "localhost:0",
			Proxies: m,
		}
		s, err := gollamas.InitService(cfg)
		assert.NoError(t, err)
		sr := gollamas.GenerateRoutes(s)
		fn(ctx, mcs, sr)
	})
}

func getRunInGinContext[I any, O any](t *testing.T, fn func(*I) *O) func(args mock.Arguments) {
	return func(args mock.Arguments) {
		c := args.Get(0).(*gin.Context)
		var req I
		gollamas.BindRequest(c, &req)
		resp := fn(&req)
		c.JSON(http.StatusOK, resp)
	}
}
