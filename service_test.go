package main_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ollama/ollama/api"
	gollamas "github.com/slawo/gollamas"
	"github.com/slawo/gollamas/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewServerFailsOnMissingContext(t *testing.T) {
	r := mocks.NewIOllamaClient(t)
	s, err := gollamas.NewService(nil, r)
	r.AssertExpectations(t)

	assert.EqualError(t, err, "missing context")
	assert.Nil(t, s)
}

func TestNewServerFailsOnMissingRouter(t *testing.T) {
	s, err := gollamas.NewService(context.TODO(), nil)

	assert.EqualError(t, err, "missing ollama client")
	assert.Nil(t, s)
}

func TestNewServerSuccess(t *testing.T) {
	r := mocks.NewIOllamaClient(t)
	s, err := gollamas.NewService(context.TODO(), r)

	assert.NoError(t, err)
	assert.NotNil(t, s)
	r.AssertExpectations(t)
}

func TestServerHome(t *testing.T) {
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	sr.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "Golamas is running", w.Body.String())

	r.AssertExpectations(t)
}

func TestServerPOSTChatMissingRequest(t *testing.T) {
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	w := httptest.NewRecorder()
	hreq, _ := http.NewRequest("POST", "/api/chat", nil)
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 400, w.Code)
	assert.Equal(t, `{"error":"invalid request"}`, w.Body.String())

	r.AssertExpectations(t)
}

var (
	TRUE        = bool(true)
	MockContext = mock.MatchedBy(func(c context.Context) bool { return true })
)

func TestServerPOSTChatRequest(t *testing.T) {
	jsonReq := []byte(`{
	"model": "some_model",
	"stream":false,
	"messages": [    {
      "role": "user",
      "content": "why is the sky blue?"
    }]}`)
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	r.On("Chat", MockContext, mock.AnythingOfType("*api.ChatRequest"), mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(1).(*api.ChatRequest)
		fn := args.Get(2).(api.ChatResponseFunc)
		assert.Equal(t, "some_model", req.Model)
		assert.NotNil(t, req.Stream)
		assert.False(t, *req.Stream)
		assert.Equal(t, 1, len(req.Messages))
		assert.Equal(t, "user", req.Messages[0].Role)
		assert.Equal(t, "why is the sky blue?", req.Messages[0].Content)
		assert.NoError(t, fn(api.ChatResponse{
			Model: "some_model",
			Message: api.Message{
				Role:    "bot",
				Content: "because it is",
			},
			Done: true,
		}))
	}).Return(nil)

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("POST", "/api/chat", bytes.NewBuffer(jsonReq))
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `{"model":"some_model","created_at":"0001-01-01T00:00:00Z","message":{"role":"bot","content":"because it is"},"done":true}`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerPOSTChatStreamRequest(t *testing.T) {
	jsonReq := []byte(`{
	"model": "some_model",
	"stream":true,
	"messages": [    {
      "role": "user",
      "content": "why is the sky blue?"
    }]}`)
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	r.On("Chat", MockContext, mock.AnythingOfType("*api.ChatRequest"), mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(1).(*api.ChatRequest)
		fn := args.Get(2).(api.ChatResponseFunc)
		assert.Equal(t, "some_model", req.Model)
		assert.NotNil(t, req.Stream)
		assert.True(t, *req.Stream)
		assert.Equal(t, 1, len(req.Messages))
		assert.Equal(t, "user", req.Messages[0].Role)
		assert.Equal(t, "why is the sky blue?", req.Messages[0].Content)
		assert.NoError(t, fn(api.ChatResponse{
			Model: "some_model",
			Message: api.Message{
				Role:    "bot",
				Content: "because",
			},
			Done: false,
		}))
		assert.NoError(t, fn(api.ChatResponse{
			Model: "some_model",
			Message: api.Message{
				Role:    "bot",
				Content: "it is",
			},
			Done: true,
		}))
	}).Return(nil)

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("POST", "/api/chat", bytes.NewBuffer(jsonReq))
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `{"model":"some_model","created_at":"0001-01-01T00:00:00Z","message":{"role":"bot","content":"because"},"done":false}
{"model":"some_model","created_at":"0001-01-01T00:00:00Z","message":{"role":"bot","content":"it is"},"done":true}
`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerPOSTCopyRequest(t *testing.T) {
	jsonReq := []byte(`{"source": "llama3.2", "destination": "llama3-backup"}`)
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("POST", "/api/copy", bytes.NewBuffer(jsonReq))
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 404, w.Code)
	assert.Equal(t, `{"error":"gollamas router doesn't support copying models"}`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerPOSTCreateBlobRequest(t *testing.T) {
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("POST", "/api/blobs/some_blob", nil)
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 404, w.Code)
	assert.Equal(t, `{"error":"gollamas router doesn't like blobs (not supported)"}`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerHEADCreateBlobRequest(t *testing.T) {
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("HEAD", "/api/blobs/some_blob", nil)
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 404, w.Code)
	assert.Equal(t, `{"error":"gollamas router doesn't like blobs (not supported)"}`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerPOSTCreateRequest(t *testing.T) {
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("POST", "/api/create", nil)
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 404, w.Code)
	assert.Equal(t, `{"error":"gollamas router doesn't support creating models"}`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerDELETEDeleteRequest(t *testing.T) {
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("DELETE", "/api/delete", nil)
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 404, w.Code)
	assert.Equal(t, `{"error":"gollamas router doesn't support deleting models"}`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerPOSTEmbeddingsRequest(t *testing.T) {
	jsonReq := []byte(`{
  "model": "all-minilm",
  "prompt": "Here is an article about llamas..."
}`)
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	ret := api.EmbeddingResponse{
		Embedding: []float64{0.5670403838157654, 0.009260174818336964, 0.23178744316101074, -0.2916173040866852, -0.8924556970596313,
			0.8785552978515625, -0.34576427936553955, 0.5742510557174683, -0.04222835972905159, -0.137906014919281},
	}
	r.On("Embeddings", MockContext, mock.AnythingOfType("*api.EmbeddingRequest")).Return(&ret, nil)

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("POST", "/api/embeddings", bytes.NewBuffer(jsonReq))
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `{"embedding":[0.5670403838157654,0.009260174818336964,0.23178744316101074,-0.2916173040866852,-0.8924556970596313,0.8785552978515625,-0.34576427936553955,0.5742510557174683,-0.04222835972905159,-0.137906014919281]}`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerPOSTEmbedRequest(t *testing.T) {
	jsonReq := []byte(`{
  "model": "all-minilm",
  "input": ["Why is the sky blue?", "Why is the grass green?"]
}`)
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	ret := api.EmbedResponse{
		Model: "all-minilm",
		Embeddings: [][]float32{
			{0.010071029, -0.0017594862, 0.05007221, 0.04692972, 0.054916814, 0.008599704, 0.105441414, -0.025878139, 0.12958129, 0.031952348},
			{-0.0098027075, 0.06042469, 0.025257962, -0.006364387, 0.07272725, 0.017194884, 0.09032035, -0.051705178, 0.09951512, 0.09072481},
		},
	}
	r.On("Embed", MockContext, mock.AnythingOfType("*api.EmbedRequest")).Return(&ret, nil)

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("POST", "/api/embed", bytes.NewBuffer(jsonReq))
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `{"model":"all-minilm","embeddings":[[0.010071029,-0.0017594862,0.05007221,0.04692972,0.054916814,0.008599704,0.105441414,-0.025878139,0.12958129,0.031952348],[-0.0098027075,0.06042469,0.025257962,-0.006364387,0.07272725,0.017194884,0.09032035,-0.051705178,0.09951512,0.09072481]]}`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerPOSTGenerateRequest(t *testing.T) {
	jsonReq := []byte(`{
  "model": "llama3.2",
  "prompt": "Why is the sky blue?",
  "stream": false
}`)
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	r.On("Generate", MockContext, mock.AnythingOfType("*api.GenerateRequest"), mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(1).(*api.GenerateRequest)
		fn := args.Get(2).(api.GenerateResponseFunc)
		assert.Equal(t, "llama3.2", req.Model)
		assert.NotNil(t, req.Stream)
		assert.False(t, *req.Stream)
		assert.Equal(t, "Why is the sky blue?", req.Prompt)
		assert.NoError(t, fn(api.GenerateResponse{
			Model:    "llama3.2",
			Response: "because it is",
			Done:     true,
		}))
	}).Return(nil)

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("POST", "/api/generate", bytes.NewBuffer(jsonReq))
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `{"model":"llama3.2","created_at":"0001-01-01T00:00:00Z","response":"because it is","done":true}`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerPOSTGenerateStreamRequest(t *testing.T) {
	jsonReq := []byte(`{
  "model": "llama3.2",
  "prompt": "Why is the sky blue?",
  "stream": true
}`)
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	r.On("Generate", MockContext, mock.AnythingOfType("*api.GenerateRequest"), mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(1).(*api.GenerateRequest)
		fn := args.Get(2).(api.GenerateResponseFunc)
		assert.Equal(t, "llama3.2", req.Model)
		assert.NotNil(t, req.Stream)
		assert.True(t, *req.Stream)
		assert.Equal(t, "Why is the sky blue?", req.Prompt)
		assert.NoError(t, fn(api.GenerateResponse{
			Model:    "llama3.2",
			Response: "because",
			Done:     false,
		}))
		assert.NoError(t, fn(api.GenerateResponse{
			Model:    "llama3.2",
			Response: "it is",
			Done:     true,
		}))
	}).Return(nil)

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("POST", "/api/generate", bytes.NewBuffer(jsonReq))
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `{"model":"llama3.2","created_at":"0001-01-01T00:00:00Z","response":"because","done":false}
{"model":"llama3.2","created_at":"0001-01-01T00:00:00Z","response":"it is","done":true}
`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerGETTagsRequest(t *testing.T) {
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	ret := api.ListResponse{
		Models: []api.ListModelResponse{
			{
				Model:  "all-minilm",
				Size:   482579485,
				Digest: "some-digest",
			},
		},
	}
	r.On("List", MockContext).Return(&ret, nil)

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("GET", "/api/tags", nil)
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `{"models":[{"name":"","model":"all-minilm","modified_at":"0001-01-01T00:00:00Z","size":482579485,"digest":"some-digest","details":{"parent_model":"","format":"","family":"","families":null,"parameter_size":"","quantization_level":""}}]}`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerGETPsRequest(t *testing.T) {
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	ret := api.ProcessResponse{
		Models: []api.ProcessModelResponse{
			{
				Model:    "all-minilm",
				Size:     482579485,
				SizeVRAM: 482579485,
				Digest:   "some-digest",
			},
		},
	}
	r.On("ListRunning", MockContext).Return(&ret, nil)

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("GET", "/api/ps", nil)
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `{"models":[{"name":"","model":"all-minilm","size":482579485,"digest":"some-digest","details":{"parent_model":"","format":"","family":"","families":null,"parameter_size":"","quantization_level":""},"expires_at":"0001-01-01T00:00:00Z","size_vram":482579485}]}`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerPOSTPullRequest(t *testing.T) {
	jsonReq := []byte(`{"model": "llama3.2", "stream": false}`)
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	//Pull(ctx context.Context, req *api.PullRequest, fn api.PullProgressFunc) error
	r.On("Pull", MockContext, mock.AnythingOfType("*api.PullRequest"), mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(1).(*api.PullRequest)
		fn := args.Get(2).(api.PullProgressFunc)
		assert.Equal(t, "llama3.2", req.Model)
		assert.NotNil(t, req.Stream)
		assert.False(t, *req.Stream)
		assert.NoError(t, fn(api.ProgressResponse{
			Status: "success",
		}))
	}).Return(nil)

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("POST", "/api/pull", bytes.NewBuffer(jsonReq))
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `{"status":"success"}`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerPOSTPullStreamRequest(t *testing.T) {
	jsonReq := []byte(`{"model": "llama3.2"}`)
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	//Pull(ctx context.Context, req *api.PullRequest, fn api.PullProgressFunc) error
	r.On("Pull", MockContext, mock.AnythingOfType("*api.PullRequest"), mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(1).(*api.PullRequest)
		fn := args.Get(2).(api.PullProgressFunc)
		assert.Equal(t, "llama3.2", req.Model)
		assert.Nil(t, req.Stream)
		assert.NoError(t, fn(api.ProgressResponse{
			Status: "pulling manifest",
			Digest: "llama3.2",
		}))
		assert.NoError(t, fn(api.ProgressResponse{
			Status:    "downloading llama3.2",
			Digest:    "llama3.2",
			Total:     43289570529,
			Completed: 43957593,
		}))
		assert.NoError(t, fn(api.ProgressResponse{
			Status:    "downloading llama3.2",
			Digest:    "llama3.2",
			Total:     43289570529,
			Completed: 985694587,
		}))
		assert.NoError(t, fn(api.ProgressResponse{
			Status:    "downloading llama3.2",
			Digest:    "llama3.2",
			Total:     43289570529,
			Completed: 28970903673,
		}))
		assert.NoError(t, fn(api.ProgressResponse{
			Status: "verifying sha256 digest",
		}))
		assert.NoError(t, fn(api.ProgressResponse{
			Status: "writing manifest",
		}))
		assert.NoError(t, fn(api.ProgressResponse{
			Status: "removing any unused layers",
		}))
		assert.NoError(t, fn(api.ProgressResponse{
			Status: "success",
		}))
	}).Return(nil)

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("POST", "/api/pull", bytes.NewBuffer(jsonReq))
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `{"status":"pulling manifest","digest":"llama3.2"}
{"status":"downloading llama3.2","digest":"llama3.2","total":43289570529,"completed":43957593}
{"status":"downloading llama3.2","digest":"llama3.2","total":43289570529,"completed":985694587}
{"status":"downloading llama3.2","digest":"llama3.2","total":43289570529,"completed":28970903673}
{"status":"verifying sha256 digest"}
{"status":"writing manifest"}
{"status":"removing any unused layers"}
{"status":"success"}
`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerPOSTPushRequest(t *testing.T) {
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("POST", "/api/push", nil)
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 404, w.Code)
	assert.Equal(t, `{"error":"gollamas router doesn't support pushing models"}`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerPOSTShowRequest(t *testing.T) {
	jsonReq := []byte(`{"model": "llama3.2"}`)
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	ret := api.ShowResponse{
		Modelfile: "llama3.2",
	}
	r.On("Show", MockContext, mock.AnythingOfType("*api.ShowRequest")).Return(&ret, nil)

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("POST", "/api/show", bytes.NewBuffer(jsonReq))
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `{"modelfile":"llama3.2","details":{"parent_model":"","format":"","family":"","families":null,"parameter_size":"","quantization_level":""},"modified_at":"0001-01-01T00:00:00Z"}`, w.Body.String())

	r.AssertExpectations(t)
}

func TestServerGETVersionRequest(t *testing.T) {
	r := mocks.NewIOllamaClient(t)
	s, _ := gollamas.NewService(context.TODO(), r)
	sr := s.GenerateRoutes()

	r.On("Version", MockContext).Return("1.1.1", nil)

	w := CreateTestResponseRecorder()
	hreq, _ := http.NewRequest("GET", "/api/version", nil)
	sr.ServeHTTP(w, hreq)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `"1.1.1"`, w.Body.String())

	r.AssertExpectations(t)
}

// VersionHandler(c *gin.Context)

type TestResponseRecorder struct {
	*httptest.ResponseRecorder
	closeChannel chan bool
}

func (r *TestResponseRecorder) CloseNotify() <-chan bool {
	return r.closeChannel
}

func (r *TestResponseRecorder) closeClient() {
	r.closeChannel <- true
}

func CreateTestResponseRecorder() *TestResponseRecorder {
	return &TestResponseRecorder{
		httptest.NewRecorder(),
		make(chan bool, 1),
	}
}
