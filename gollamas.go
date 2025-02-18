package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/ollama/ollama/api"
	"github.com/ollama/ollama/envconfig"
	"github.com/ollama/ollama/openai"
	"github.com/ollama/ollama/types/errtypes"
	"github.com/ollama/ollama/types/model"
	log "github.com/sirupsen/logrus"
)

type ProxyConfig struct {
	url string
}

// NewService instantiates a new instance of the [Service].
// Hail to the llamas!
func NewService(ctx context.Context, cfg map[string]ProxyConfig) (*Service, error) {
	cmap, err := initClients(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Service{
		cfg:  cfg,
		cmap: cmap,
	}, nil
}

type Service struct {
	cfg  map[string]ProxyConfig
	cmap map[string]*api.Client
}

func (s *Service) GenerateRoutes() http.Handler {
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
	r.POST("/api/pull", s.PullHandler)
	r.POST("/api/generate", s.GenerateHandler)
	r.POST("/api/chat", s.ChatHandler)
	r.POST("/api/embed", s.EmbedHandler)
	r.POST("/api/embeddings", s.EmbeddingsHandler)
	r.POST("/api/create", s.CreateHandler)
	r.POST("/api/push", s.PushHandler)
	r.POST("/api/copy", s.CopyHandler)
	r.DELETE("/api/delete", s.DeleteHandler)
	r.POST("/api/show", s.ShowHandler)
	r.POST("/api/blobs/:digest", s.CreateBlobHandler)
	r.HEAD("/api/blobs/:digest", s.HeadBlobHandler)
	r.GET("/api/ps", s.PsHandler)

	// Compatibility endpoints
	r.POST("/v1/chat/completions", openai.ChatMiddleware(), s.ChatHandler)
	r.POST("/v1/completions", openai.CompletionsMiddleware(), s.GenerateHandler)
	r.POST("/v1/embeddings", openai.EmbeddingsMiddleware(), s.EmbedHandler)
	r.GET("/v1/models", openai.ListMiddleware(), s.ListHandler)
	r.GET("/v1/models/:model", openai.RetrieveMiddleware(), s.ShowHandler)

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		r.Handle(method, "/", func(c *gin.Context) {
			c.String(http.StatusOK, "Ollama is running")
		})

		r.Handle(method, "/api/tags", s.ListHandler)
		r.Handle(method, "/api/version", s.VersionHandler)
	}

	return r

}

func (s *Service) PullHandler(c *gin.Context) {
	var req api.PullRequest
	if !s.bindRequest(c, &req) {
		return
	}
	cl := s.getClientByModel(c, cmp.Or(req.Model, req.Name))
	if cl == nil {
		return
	}

	log.WithField("name", cmp.Or(req.Model, req.Name)).Info("Will pull model")
	ch := make(chan any)
	go func() {
		defer close(ch)
		cl.Pull(c.Request.Context(), &req, func(pr api.ProgressResponse) error {
			ch <- pr
			return nil
		})
	}()
	if req.Stream != nil && !*req.Stream {
		waitForStream(c, ch)
		return
	}
	streamResponse(c, ch)
}

func (s *Service) GenerateHandler(c *gin.Context) {
	var req api.GenerateRequest
	if !s.bindRequest(c, &req) {
		return
	}
	cl := s.getClientByModel(c, req.Model)
	if cl == nil {
		return
	}
	ch := make(chan any)
	go func() {
		defer close(ch)
		cl.Generate(c.Request.Context(), &req, func(gr api.GenerateResponse) error {
			ch <- gr
			return nil
		})
	}()

	if req.Stream != nil && !*req.Stream {
		var r api.GenerateResponse
		var sb strings.Builder
		for rr := range ch {
			switch t := rr.(type) {
			case api.GenerateResponse:
				sb.WriteString(t.Response)
				r = t
			case gin.H:
				msg, ok := t["error"].(string)
				if !ok {
					msg = "unexpected error format in response"
				}

				c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
				return
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected response"})
				return
			}
		}

		r.Response = sb.String()
		c.JSON(http.StatusOK, r)
		return
	}
	streamResponse(c, ch)
}

func (s *Service) ChatHandler(c *gin.Context) {
	var req api.ChatRequest
	if !s.bindRequest(c, &req) {
		return
	}
	cl := s.getClientByModel(c, req.Model)
	if cl == nil {
		return
	}

	ch := make(chan any)
	go func() {
		defer close(ch)
		cl.Chat(c.Request.Context(), &req, func(pr api.ChatResponse) error {
			ch <- pr
			return nil
		})
	}()
	if req.Stream != nil && !*req.Stream {
		waitForStream(c, ch)
		return
	}
	streamResponse(c, ch)
}

func (s *Service) EmbedHandler(c *gin.Context) {
	var req api.EmbedRequest
	if !s.bindRequest(c, &req) {
		return
	}
	cl := s.getClientByModel(c, req.Model)
	if cl == nil {
		return
	}
	resp, err := cl.Embed(c.Request.Context(), &req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Service) EmbeddingsHandler(c *gin.Context) {
	var req api.EmbeddingRequest
	if !s.bindRequest(c, &req) {
		return
	}
	cl := s.getClientByModel(c, req.Model)
	if cl == nil {
		return
	}
	resp, err := cl.Embeddings(c.Request.Context(), &req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
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
	var req api.ShowRequest
	if !s.bindRequest(c, &req) {
		return
	}
	cl := s.getClientByModel(c, cmp.Or(req.Model, req.Name))
	if cl == nil {
		return
	}
	resp, err := cl.Show(c.Request.Context(), &req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Service) CreateBlobHandler(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "gollamas router doesn't like blobs (not supported)"})
}

func (s *Service) HeadBlobHandler(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "gollamas router doesn't like blobs (not supported)"})
}

func (s *Service) PsHandler(c *gin.Context) {
	ch := make(chan *api.ProcessResponse)
	wg := sync.WaitGroup{}
	for _, v := range s.cmap {
		wg.Add(1)
		go func(cl *api.Client) {
			defer wg.Done()
			v, err := cl.ListRunning(c.Request.Context())
			if err != nil {
				log.WithError(err).Errorf("Failed to retrieve running models")
			}
			ch <- v
		}(v)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	var res api.ProcessResponse
	for pr := range ch {
		res.Models = append(res.Models, pr.Models...)
	}
	c.JSON(http.StatusOK, res)
}

func (s *Service) ListHandler(c *gin.Context) {
	ch := make(chan *api.ListResponse)
	wg := sync.WaitGroup{}
	for _, v := range s.cmap {
		wg.Add(1)
		go func(cl *api.Client) {
			defer wg.Done()
			v, err := cl.List(c.Request.Context())
			if err != nil {
				log.WithError(err).Errorf("Failed to retrieve running models")
			}
			ch <- v
		}(v)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	var res api.ListResponse
	for lr := range ch {
		res.Models = append(res.Models, lr.Models...)
	}
	slices.SortStableFunc(res.Models, func(i, j api.ListModelResponse) int {
		// most recently modified first
		return cmp.Compare(j.ModifiedAt.Unix(), i.ModifiedAt.Unix())
	})
	c.JSON(http.StatusOK, res)
}

func (s *Service) VersionHandler(c *gin.Context) {
	ch := make(chan string)
	wg := sync.WaitGroup{}
	for _, v := range s.cmap {
		wg.Add(1)
		go func(cl *api.Client) {
			defer wg.Done()
			v, err := cl.Version(c.Request.Context())
			if err != nil {
				log.WithError(err).Errorf("Failed to retrieve version")
			}
			ch <- v
		}(v)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	v := ""
	for i := range ch {
		if v == "" || i < v {
			v = i
		}
	}
	c.JSON(http.StatusOK, gin.H{"version": v})
}

func (s *Service) bindRequest(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); errors.Is(err, io.EOF) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing request body"})
		return false
	} else if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return false
	}
	return true
}

func (s *Service) getClientByModel(c *gin.Context, m string) *api.Client {
	name := model.ParseName(m)
	if !name.IsValid() {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": errtypes.InvalidModelNameErrMsg})
		return nil
	}
	cl, ok := s.cmap[name.DisplayShortest()]
	if !ok {
		log.WithField("name", name).Errorf("Client for model not found")
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "gollamas router is missing a valid route to model"})
		return nil
	}
	return cl
}

// shamelessly copied from https://raw.githubusercontent.com/ollama/ollama/refs/tags/v0.5.11/server/routes.go
func waitForStream(c *gin.Context, ch chan interface{}) {
	c.Header("Content-Type", "application/json")
	for resp := range ch {
		switch r := resp.(type) {
		case api.ProgressResponse:
			if r.Status == "success" {
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

func initClients(ctx context.Context, pc map[string]ProxyConfig) (map[string]*api.Client, error) {
	cmap := map[string]*api.Client{}
	for k, v := range pc {
		l := log.WithField("server", v.url)
		remote, err := url.Parse(v.url)
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
