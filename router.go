package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sync"

	"github.com/ollama/ollama/api"
	"github.com/ollama/ollama/types/errtypes"
	"github.com/ollama/ollama/types/model"
	log "github.com/sirupsen/logrus"
)

type ProxyConfig struct {
	Url string
}

// NewRouter creates a new router
func NewRouter(ctx context.Context, cmap map[string]IOllamaClient) (*Router, error) {
	if ctx == nil {
		return nil, errors.New("missing context")
	}
	if cmap == nil {
		return nil, errors.New("missing ollama client map")
	}
	if len(cmap) == 0 {
		return nil, errors.New("empty ollama client map")
	}
	clids := make(map[string]IOllamaClient, len(cmap))
	for id, cl := range cmap {
		if cl == nil {
			return nil, fmt.Errorf("nil client for model %s", id)
		}
		name := model.ParseName(id)
		clids[name.DisplayShortest()] = cl
	}
	return &Router{
		cmap: clids,
	}, nil
}

// Router is a router that routes requests to the appropriate client
type Router struct {
	cmap map[string]IOllamaClient
}

func (r *Router) Chat(ctx context.Context, req *api.ChatRequest, fn api.ChatResponseFunc) error {
	cl, err := r.getClientByModel(ctx, req.Model)
	if err != nil {
		return err
	}
	return cl.Chat(ctx, req, fn)
}

func (r *Router) Copy(ctx context.Context, req *api.CopyRequest) error {
	return NewHttpError(http.StatusNotFound, "gollamas: router doesn't support copying models")
}

func (r *Router) Create(ctx context.Context, req *api.CreateRequest, fn api.CreateProgressFunc) error {
	return NewHttpError(http.StatusNotFound, "gollamas: router doesn't support creating models")
}

func (r *Router) CreateBlob(ctx context.Context, digest string, rd io.Reader) error {
	return NewHttpError(http.StatusNotFound, "gollamas: router doesn't like blobs (not supported)")
}

func (r *Router) Delete(ctx context.Context, req *api.DeleteRequest) error {
	return NewHttpError(http.StatusNotFound, "gollamas: router doesn't support deleting models")
}

func (r *Router) Embed(ctx context.Context, req *api.EmbedRequest) (*api.EmbedResponse, error) {
	cl, err := r.getClientByModel(ctx, req.Model)
	if err != nil {
		return nil, err
	}
	return cl.Embed(ctx, req)
}

func (r *Router) Embeddings(ctx context.Context, req *api.EmbeddingRequest) (*api.EmbeddingResponse, error) {
	cl, err := r.getClientByModel(ctx, req.Model)
	if err != nil {
		return nil, err
	}
	return cl.Embeddings(ctx, req)
}

func (r *Router) Generate(ctx context.Context, req *api.GenerateRequest, fn api.GenerateResponseFunc) error {
	cl, err := r.getClientByModel(ctx, req.Model)
	if err != nil {
		return err
	}
	return cl.Generate(ctx, req, fn)
}

func (r *Router) Heartbeat(ctx context.Context) error {
	type idErr struct {
		id  string
		err error
	}
	ch := make(chan idErr)
	wg := sync.WaitGroup{}
	for id, cl := range r.cmap {
		wg.Add(1)
		go func(id string, cl IOllamaClient) {
			defer wg.Done()
			err := cl.Heartbeat(ctx)
			ch <- idErr{id: id, err: err}
		}(id, cl)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	var err error
	for r := range ch {
		if r.err != nil {
			errors.Join(err, r.err)
		}
	}
	return err
}

func (r *Router) List(ctx context.Context) (*api.ListResponse, error) {
	ch := make(chan *api.ListResponse)
	wg := sync.WaitGroup{}
	for id, v := range r.cmap {
		wg.Add(1)
		go func(id string, cl IOllamaClient) {
			defer wg.Done()
			v, err := cl.List(ctx)
			if err != nil {
				log.WithField("id", id).WithError(err).Errorf("Failed to retrieve running models")
			}
			ch <- v
		}(id, v)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	var res api.ListResponse
	for lr := range ch {
		if lr != nil && lr.Models != nil {
			res.Models = append(res.Models, lr.Models...)
		}
	}
	slices.SortStableFunc(res.Models, func(i, j api.ListModelResponse) int {
		// most recently modified first
		return cmp.Compare(j.ModifiedAt.Unix(), i.ModifiedAt.Unix())
	})
	return &res, nil
}

func (r *Router) ListRunning(ctx context.Context) (*api.ProcessResponse, error) {
	type rsp struct {
		v *api.ProcessResponse
		e error
	}
	ch := make(chan *rsp)
	wg := sync.WaitGroup{}
	for id, v := range r.cmap {
		wg.Add(1)
		go func(id string, cl IOllamaClient) {
			defer wg.Done()
			v, err := cl.ListRunning(ctx)
			if err != nil {
				log.WithField("id", id).WithError(err).Errorf("Failed to retrieve running models")
			}
			ch <- &rsp{
				v: v,
				e: err,
			}
		}(id, v)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	var res api.ProcessResponse
	for pr := range ch {
		if pr != nil && pr.v != nil {
			res.Models = append(res.Models, pr.v.Models...)
		}
	}
	return &res, nil
}

func (r *Router) Push(ctx context.Context, req *api.PushRequest, fn api.PushProgressFunc) error {
	return NewHttpError(http.StatusNotFound, "gollamas: router doesn't support pushing models")
}

func (r *Router) Pull(ctx context.Context, req *api.PullRequest, fn api.PullProgressFunc) error {
	cl, err := r.getClientByModel(ctx, cmp.Or(req.Model, req.Name))
	if err != nil {
		return err
	}
	return cl.Pull(ctx, req, fn)
}

func (r *Router) Show(ctx context.Context, req *api.ShowRequest) (*api.ShowResponse, error) {
	cl, err := r.getClientByModel(ctx, cmp.Or(req.Model, req.Name))
	if err != nil {
		return nil, err
	}
	return cl.Show(ctx, req)
}

func (r *Router) Version(ctx context.Context) (string, error) {
	ch := make(chan string)
	wg := sync.WaitGroup{}
	for id, v := range r.cmap {
		wg.Add(1)
		go func(id string, cl IOllamaClient) {
			defer wg.Done()
			v, err := cl.Version(ctx)
			if err != nil {
				log.WithField("id", id).WithError(err).Errorf("Failed to retrieve version")
			}
			ch <- v
		}(id, v)
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
	return v, nil
}

func (r *Router) getClientByModel(ctx context.Context, m string) (IOllamaClient, error) {
	name := model.ParseName(m)
	if !name.IsValid() {
		log.WithField("name", name).Errorf("Invalid model name")
		return nil, NewHttpError(http.StatusBadRequest, errtypes.InvalidModelNameErrMsg)
	}
	cl, ok := r.cmap[name.DisplayShortest()]
	if !ok {
		return nil, NewHttpError(http.StatusNotFound, "gollamas router is missing a valid route to model")
	}
	return cl, nil
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
