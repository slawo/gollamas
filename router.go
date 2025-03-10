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
	"github.com/ollama/ollama/types/model"
	log "github.com/sirupsen/logrus"
)

type ProxyConfig struct {
	Url string
}

// NewRouter creates a new router
func NewRouter(cmap map[string]IOllamaClient, opts ...RouterOption) (*Router, error) {
	if cmap == nil {
		return nil, errors.New("missing ollama client map")
	}
	if len(cmap) == 0 {
		return nil, errors.New("empty ollama client map")
	}
	clids := make(map[string]IOllamaClient, len(cmap))
	all2ModelID := map[string]string{}
	for id, cl := range cmap {
		if cl == nil {
			return nil, fmt.Errorf("nil client for model %s", id)
		}
		name := model.ParseName(id)
		if !name.IsValid() {
			return nil, fmt.Errorf("invalid model name: %s", id)
		}
		clids[id] = cl
		all2ModelID[id] = id
	}
	opt := RouterOptions{ExposeAliases: true}
	for _, o := range opts {
		if err := o.ApplyTo(&opt); err != nil {
			return nil, err
		}
	}
	r := &Router{
		cmap:          clids,
		all2ModelID:   all2ModelID,
		exposeAliases: opt.ExposeAliases,
	}
	if err := r.setAliases(opt.Aliases); err != nil {
		return nil, err
	}
	return r, nil
}

// Router is a router that routes requests to the appropriate client
type Router struct {
	exposeAliases bool
	alias2model   map[string]string
	model2aliases map[string][]string
	cmap          map[string]IOllamaClient
	all2ModelID   map[string]string // this is a temporary map of all possible names with the id of the connection
}

func (r *Router) Chat(ctx context.Context, req *api.ChatRequest, fn api.ChatResponseFunc) error {
	cl, m, err := r.getClientAndModelByModelName(req.Model)
	if err != nil {
		return err
	}
	req.Model = m
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
	cl, m, err := r.getClientAndModelByModelName(req.Model)
	if err != nil {
		return nil, err
	}
	req.Model = m
	return cl.Embed(ctx, req)
}

func (r *Router) Embeddings(ctx context.Context, req *api.EmbeddingRequest) (*api.EmbeddingResponse, error) {
	cl, m, err := r.getClientAndModelByModelName(req.Model)
	if err != nil {
		return nil, err
	}
	req.Model = m
	return cl.Embeddings(ctx, req)
}

func (r *Router) Generate(ctx context.Context, req *api.GenerateRequest, fn api.GenerateResponseFunc) error {
	cl, m, err := r.getClientAndModelByModelName(req.Model)
	if err != nil {
		return err
	}
	req.Model = m
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
		errors.Join(err, r.err)
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
				log.WithField("id", id).WithError(err).Errorf("Failed to retrieve models.")
			}
			if v != nil {
				v.Models = r.filterListToMapedModels(v.Models, id)
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

func (r *Router) filterListToMapedModels(orig []api.ListModelResponse, ids ...string) []api.ListModelResponse {
	idsmap := map[string]string{}
	for _, id := range ids {
		idsmap[id] = id
		name := model.ParseName(id)
		idsmap[name.DisplayShortest()] = id
	}
	log.WithField("ids_map", idsmap).Trace("filterListToMapedModels.")
	var res []api.ListModelResponse
	for _, m := range orig {
		// both name and model are name.DisplayShortest() in the ollama api we use model for consistency
		log.WithField("name", m.Name).WithField("model", m.Model).Trace("Filtering model.")
		if id, ok := idsmap[m.Model]; ok {
			res = append(res, m)
			if r.exposeAliases && len(r.model2aliases[id]) > 0 {
				for _, alias := range r.model2aliases[id] {
					res = append(res, api.ListModelResponse{
						Name:       alias,
						Model:      alias,
						ModifiedAt: m.ModifiedAt,
						Size:       m.Size,
						Digest:     m.Digest,
						Details:    m.Details,
					})
				}
			}
		} else {
			log.WithField("name", m.Name).WithField("model", m.Model).Trace("Model has been filtered out of response.")
		}
	}
	return res
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
				log.WithField("id", id).WithError(err).Errorf("Failed to retrieve running models.")
			}
			if v != nil {
				v.Models = r.filterRunningListToMapedModels(v.Models, id)
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
	slices.SortStableFunc(res.Models, func(i, j api.ProcessModelResponse) int {
		// sort by name
		return -1 * cmp.Compare(j.Name, i.Name)
	})
	return &res, nil
}

func (r *Router) filterRunningListToMapedModels(orig []api.ProcessModelResponse, ids ...string) []api.ProcessModelResponse {
	idsmap := map[string]string{}
	for _, id := range ids {
		idsmap[id] = id
		name := model.ParseName(id)
		idsmap[name.DisplayShortest()] = id
	}
	log.WithField("ids_map", idsmap).Trace("filterRunningListToMapedModels.")
	var res []api.ProcessModelResponse
	for _, m := range orig {
		log.WithField("name", m.Name).WithField("model", m.Model).Trace("Filtering model.")
		if id, ok := idsmap[m.Model]; ok {
			res = append(res, m)
			if r.exposeAliases && len(r.model2aliases[id]) > 0 {
				for _, alias := range r.model2aliases[id] {
					res = append(res, api.ProcessModelResponse{
						Name:      alias,
						Model:     alias,
						Size:      m.Size,
						Digest:    m.Digest,
						Details:   m.Details,
						ExpiresAt: m.ExpiresAt,
						SizeVRAM:  m.SizeVRAM,
					})
				}
			}
		} else {
			log.WithField("name", m.Name).WithField("model", m.Model).Trace("Model has been filtered out of response.")
		}
	}
	return res
}

func (r *Router) Pull(ctx context.Context, req *api.PullRequest, fn api.PullProgressFunc) error {
	cl, m, err := r.getClientAndModelByModelName(cmp.Or(req.Model, req.Name))
	if err != nil {
		return err
	}
	if req.Model == "" {
		req.Name = m
	} else {
		req.Model = m
	}
	return cl.Pull(ctx, req, fn)
}

func (r *Router) Push(ctx context.Context, req *api.PushRequest, fn api.PushProgressFunc) error {
	return NewHttpError(http.StatusNotFound, "gollamas: router doesn't support pushing models")
}

func (r *Router) Show(ctx context.Context, req *api.ShowRequest) (*api.ShowResponse, error) {
	cl, m, err := r.getClientAndModelByModelName(cmp.Or(req.Model, req.Name))
	if err != nil {
		return nil, err
	}
	if req.Model == "" {
		req.Name = m
	} else {
		req.Model = m
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
				log.WithField("id", id).WithError(err).Errorf("Failed to retrieve version.")
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

func (r *Router) setAliases(aliases map[string]string) error {
	if len(aliases) == 0 {
		return nil
	}
	if r.alias2model == nil {
		r.alias2model = map[string]string{}
	}
	if r.model2aliases == nil {
		r.model2aliases = map[string][]string{}
	}
	for k, v := range aliases {
		if err := r.addAlias(k, v); err != nil {
			return err
		}
	}
	return nil
}

func (r *Router) addAlias(alias, model string) error {
	if _, ok := r.cmap[model]; !ok {
		return fmt.Errorf("alias %s points to unknown model %s", alias, model)
	}
	if _, ok := r.cmap[alias]; ok {
		return fmt.Errorf("alias %s refers to an existing concrete model name", alias)
	}
	r.alias2model[alias] = model
	r.model2aliases[model] = append(r.model2aliases[model], alias)
	return nil
}

func (r *Router) getClientAndModelByModelName(requested string) (IOllamaClient, string, error) {
	log.WithField("requested_model", requested).Trace("Routing: request.")
	modelID, ok := r.all2ModelID[requested]
	if !ok {
		log.WithField("requested_model", requested).Trace("Routing: no direct route to model.")
		if alias, ok := r.alias2model[requested]; ok {
			log.WithField("modelID", alias).WithField("requested_model", requested).Trace("Routing: selected model.")
			modelID = alias
		} else {
			log.WithField("requested_model", requested).Trace("Routing: no route to model.")
		}
	}
	cl := r.cmap[modelID]
	if cl == nil {
		if modelID != "" && modelID != requested {
			return nil, requested, NewHttpErrorf(http.StatusNotFound, "gollamas router is missing a valid route to model %s (%s)", requested, modelID)
		}
		return nil, requested, NewHttpErrorf(http.StatusNotFound, "gollamas router is missing a valid route to model %s", requested)
	}

	return cl, modelID, nil
}

func initClients(pc map[string]ProxyConfig) (map[string]IOllamaClient, error) {
	if pc == nil {
		return nil, errors.New("missing proxy config")
	}
	if len(pc) == 0 {
		return nil, errors.New("empty proxy config map")
	}
	cmap := map[string]IOllamaClient{}
	for k, v := range pc {
		remote, err := url.Parse(v.Url)
		if err != nil {
			return nil, err
		}
		client := api.NewClient(remote, http.DefaultClient)
		name := model.ParseName(k)
		if !name.IsValid() {
			return nil, fmt.Errorf("invalid model name: %s", k)
		}
		cmap[k] = client
	}

	return cmap, nil
}

func initRouterAliasOpts(aliases map[string]string) []RouterOption {
	return []RouterOption{&RouterOptions{
		Aliases: aliases,
	}}
}
