package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/ollama/ollama/api"
	"github.com/ollama/ollama/envconfig"
	"github.com/ollama/ollama/openai"
	"github.com/ollama/ollama/types/model"
	log "github.com/sirupsen/logrus"
)

type ProxyConfig struct {
	Url string
}

func NewHttpError(code int, message string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
	}
}

type HttpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *HttpError) Error() string {
	return e.Message
}

func (e *HttpError) StatusCode() int {
	return e.Code
}

// NewService instantiates a new instance of the [Service].
// Hail to the llamas!
func NewService(ctx context.Context, r IOllamaClient) (*Service, error) {
	if ctx == nil {
		return nil, errors.New("missing context")
	}
	if r == nil {
		return nil, errors.New("missing ollama client")
	}
	return &Service{
		r: r,
	}, nil
}

//go:generate mockery --name IOllamaClient --output mocks
type IOllamaClient interface {
	Generate(ctx context.Context, req *api.GenerateRequest, fn api.GenerateResponseFunc) error
	Chat(ctx context.Context, req *api.ChatRequest, fn api.ChatResponseFunc) error
	Pull(ctx context.Context, req *api.PullRequest, fn api.PullProgressFunc) error
	Push(ctx context.Context, req *api.PushRequest, fn api.PushProgressFunc) error
	Create(ctx context.Context, req *api.CreateRequest, fn api.CreateProgressFunc) error
	List(ctx context.Context) (*api.ListResponse, error)
	ListRunning(ctx context.Context) (*api.ProcessResponse, error)
	Copy(ctx context.Context, req *api.CopyRequest) error
	Delete(ctx context.Context, req *api.DeleteRequest) error
	Show(ctx context.Context, req *api.ShowRequest) (*api.ShowResponse, error)
	Heartbeat(ctx context.Context) error
	Embed(ctx context.Context, req *api.EmbedRequest) (*api.EmbedResponse, error)
	Embeddings(ctx context.Context, req *api.EmbeddingRequest) (*api.EmbeddingResponse, error)
	CreateBlob(ctx context.Context, digest string, r io.Reader) error
	Version(ctx context.Context) (string, error)
}

//go:generate mockery --name IGinService --output mocks
type IGinService interface {
	ChatHandler(c *gin.Context)
	CopyHandler(c *gin.Context)
	CreateBlobHandler(c *gin.Context)
	CreateHandler(c *gin.Context)
	DeleteHandler(c *gin.Context)
	EmbeddingsHandler(c *gin.Context)
	EmbedHandler(c *gin.Context)
	GenerateHandler(c *gin.Context)
	HeadBlobHandler(c *gin.Context)
	HomeHandler(c *gin.Context)
	ListHandler(c *gin.Context)
	PsHandler(c *gin.Context)
	PullHandler(c *gin.Context)
	PushHandler(c *gin.Context)
	ShowHandler(c *gin.Context)
	VersionHandler(c *gin.Context)
}

type Service struct {
	r IOllamaClient
}

func (s *Service) GenerateRoutes() *gin.Engine {
	config := cors.DefaultConfig()
	config.AllowWildcard = true
	config.AllowBrowserExtensions = true
	config.AllowHeaders = []string{"Authorization", "Content-Type", "User-Agent", "Accept", "X-Requested-With"}
	openAIProperties := []string{"lang", "package-version", "os", "arch", "retry-count", "runtime", "runtime-version", "async", "helper-method", "poll-helper", "custom-poll-interval"}
	for _, prop := range openAIProperties {
		config.AllowHeaders = append(config.AllowHeaders, "x-stainless-"+prop)
	}
	config.AllowOrigins = envconfig.Origins()

	r := gin.Default()
	r.Use(
		cors.New(config),
	)

	// refer to https://github.com/ollama/ollama/blob/0667baddc658d3f556a369701819e7695477f59a/server/routes.go#L1146
	// for the routes and setup in this file
	r.DELETE("/api/delete", s.DeleteHandler)

	r.GET("/", s.HomeHandler)
	r.GET("/api/ps", s.PsHandler)
	r.GET("/api/tags", s.ListHandler)
	r.GET("/api/version", s.VersionHandler)
	r.GET("/v1/models", openai.ListMiddleware(), s.ListHandler)
	r.GET("/v1/models/:model", openai.RetrieveMiddleware(), s.ShowHandler)

	r.HEAD("/", s.HomeHandler)
	r.HEAD("/api/blobs/:digest", s.HeadBlobHandler)
	r.HEAD("/v1/models", openai.ListMiddleware(), s.ListHandler)
	r.HEAD("/v1/models/:model", openai.RetrieveMiddleware(), s.ShowHandler)

	r.POST("/api/chat", s.ChatHandler)
	r.POST("/api/copy", s.CopyHandler)
	r.POST("/api/create", s.CreateHandler)
	r.POST("/api/generate", s.GenerateHandler)
	r.POST("/api/embed", s.EmbedHandler)
	r.POST("/api/embeddings", s.EmbeddingsHandler)
	r.POST("/api/pull", s.PullHandler)
	r.POST("/api/push", s.PushHandler)
	r.POST("/api/show", s.ShowHandler)
	r.POST("/api/blobs/:digest", s.CreateBlobHandler)

	// Compatibility endpoints
	r.POST("/v1/chat/completions", openai.ChatMiddleware(), s.ChatHandler)
	r.POST("/v1/completions", openai.CompletionsMiddleware(), s.GenerateHandler)
	r.POST("/v1/embeddings", openai.EmbeddingsMiddleware(), s.EmbedHandler)

	return r
}

func (s *Service) HomeHandler(c *gin.Context) {
	c.String(http.StatusOK, "Golamas is running")
}

func (s *Service) PullHandler(c *gin.Context) {
	handleStreamRequest(c, s.r.Pull)
}

func (s *Service) GenerateHandler(c *gin.Context) {
	handleStreamRequest(c, s.r.Generate)
}

func (s *Service) ChatHandler(c *gin.Context) {
	handleStreamRequest(c, s.r.Chat)
}

func (s *Service) EmbedHandler(c *gin.Context) {
	handleRequest(c, s.r.Embed)
}

func (s *Service) EmbeddingsHandler(c *gin.Context) {
	handleRequest(c, s.r.Embeddings)
}

func (s *Service) CreateHandler(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "gollamas router doesn't support creating models"})
}

func (s *Service) PushHandler(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "gollamas router doesn't support pushing models"})
}

func (s *Service) CopyHandler(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "gollamas router doesn't support copying models"})
}

func (s *Service) DeleteHandler(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "gollamas router doesn't support deleting models"})
}

func (s *Service) ShowHandler(c *gin.Context) {
	handleRequest(c, s.r.Show)
}

func (s *Service) CreateBlobHandler(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "gollamas router doesn't like blobs (not supported)"})
}

func (s *Service) HeadBlobHandler(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "gollamas router doesn't like blobs (not supported)"})
}

func (s *Service) PsHandler(c *gin.Context) {
	handle(c, s.r.ListRunning)
}

func (s *Service) ListHandler(c *gin.Context) {
	handle(c, s.r.List)
}

func (s *Service) VersionHandler(c *gin.Context) {
	handle(c, s.r.Version)
}

func bindRequest(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); errors.Is(err, io.EOF) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing request body"})
		return false
	} else if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return false
	}
	return true
}

func handle[R any](c *gin.Context, fn func(context.Context) (R, error)) {
	resp, err := fn(c.Request.Context())
	if err != nil {
		abortGinError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func handleRequest[T any, R any](c *gin.Context, fn func(context.Context, *T) (R, error)) {
	var req T
	if !bindRequest(c, &req) {
		return
	}
	resp, err := fn(c.Request.Context(), &req)
	if err != nil {
		abortGinError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func handleStreamRequest[T any, R any, F ~func(R) error](c *gin.Context, fn func(context.Context, *T, F) error) {
	var req T
	if !bindRequest(c, &req) {
		return
	}
	ch := make(chan any)
	go func() {
		defer func(ch chan any) {
			close(ch)
		}(ch)
		fn(c.Request.Context(), &req, func(pr R) error {
			ch <- pr
			return nil
		})
	}()
	b, err := extractBoolPointerFromRequest(&req)
	if err != nil {
		abortGinError(c, err)
		return
	}
	if b != nil && !*b {
		waitForStream(c, ch)
		return
	}
	streamResponse(c, ch)
}

// shamelessly copied from https://raw.githubusercontent.com/ollama/ollama/refs/tags/v0.5.11/server/routes.go
func waitForStream(c *gin.Context, ch chan interface{}) {
	c.Header("Content-Type", "application/json")
	for resp := range ch {
		switch r := resp.(type) {
		case api.ChatResponse:
			if r.Done {
				c.JSON(http.StatusOK, r)
				return
			}
		case api.ProgressResponse:
			if r.Status == "success" {
				c.JSON(http.StatusOK, r)
				return
			}
		case api.GenerateResponse:
			if r.Done {
				c.JSON(http.StatusOK, r)
				return
			}
		case gin.H:
			status, ok := r["status"].(int)
			if !ok {
				status = http.StatusInternalServerError
			}
			if errorMsg, ok := r["error"].(string); ok {
				c.JSON(status, gin.H{"error": errorMsg})
				return
			} else {
				c.JSON(status, gin.H{"error": "unexpected error format in progress response"})
				return
			}
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected progress response"})
			return
		}
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected end of progress response"})
}

// shamelessly copied from https://raw.githubusercontent.com/ollama/ollama/refs/tags/v0.5.11/server/routes.go
func streamResponse(c *gin.Context, ch chan any) {
	c.Header("Content-Type", "application/x-ndjson")
	c.Stream(func(w io.Writer) bool {
		val, ok := <-ch
		if !ok {
			return false
		}

		bts, err := json.Marshal(val)
		if err != nil {
			slog.Info(fmt.Sprintf("streamResponse: json.Marshal failed with %s", err))
			return false
		}

		// Delineate chunks with new-line delimiter
		bts = append(bts, '\n')
		if _, err := w.Write(bts); err != nil {
			slog.Info(fmt.Sprintf("streamResponse: w.Write failed with %s", err))
			return false
		}

		return true
	})
}

func initClients(ctx context.Context, pc map[string]ProxyConfig) (map[string]IOllamaClient, error) {
	if ctx == nil {
		return nil, errors.New("missing context")
	}
	if pc == nil {
		return nil, errors.New("missing proxy config")
	}
	if len(pc) == 0 {
		return nil, errors.New("empty proxy config map")
	}
	cmap := map[string]IOllamaClient{}
	for k, v := range pc {
		l := log.WithField("server", v.Url)
		remote, err := url.Parse(v.Url)
		if err != nil {
			return nil, err
		}
		client := api.NewClient(remote, http.DefaultClient)

		name := model.ParseName(k)
		if !name.IsValid() {
			return nil, fmt.Errorf("invalid model name: %s", k)
		}
		cmap[name.DisplayShortest()] = client

		version, err := client.Version(ctx)
		if err == nil {
			l.WithField("version", version).Tracef("Connected to server.")
		}
	}

	return cmap, nil
}

func abortGinError(c *gin.Context, err error) {
	var httpErr *HttpError
	if errors.As(err, &httpErr) {
		c.AbortWithStatusJSON(httpErr.StatusCode(), gin.H{"error": httpErr.Error()})
		return
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
