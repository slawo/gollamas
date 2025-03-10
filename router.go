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

type ModelConfig struct {
	ConnectionID string
}

// NewRouter creates a new router
func NewRouter(cmap map[string]IOllamaClient, mconf map[string]ModelConfig, opts ...RouterOption) (*Router, error) {
	if cmap == nil {
		return nil, errors.New("missing ollama client map")
	}
	if len(cmap) == 0 {
		return nil, errors.New("empty ollama client map")
	}
	clids := make(map[string]IOllamaClient, len(cmap))
	all2ModelID := map[string]string{}
	cids2models := map[string][]string{}

	for id, cl := range cmap {
		if cl == nil {
			return nil, fmt.Errorf("nil client for connection id %s", id)
		}
		clids[id] = cl
		cids2models[id] = []string{}
	}
	for id, mc := range mconf {
		if mc.ConnectionID == "" {
			return nil, fmt.Errorf("empty connection id for model %s", id)
		}
		if _, ok := clids[mc.ConnectionID]; !ok {
			return nil, fmt.Errorf("unknown connection id for model %s", id)
		}
		cids2models[mc.ConnectionID] = append(cids2models[mc.ConnectionID], id)
		name := model.ParseName(id)
		if !name.IsValid() {
			return nil, fmt.Errorf("invalid model name: %s", id)
		}
		all2ModelID[id] = id
		all2ModelID[name.DisplayShortest()] = id
	}
	opt := RouterOptions{ExposeAliases: true}
	for _, o := range opts {
		if err := o.ApplyTo(&opt); err != nil {
			return nil, err
		}
	}
	r := &Router{
		cmap:          clids,
		modelCfg:      mconf,
		cids2models:   cids2models,
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
	modelCfg      map[string]ModelConfig
	cmap          map[string]IOllamaClient
	cids2models   map[string][]string
	all2ModelID   map[string]string // this is a temporary map of all possible names with the id of the connection
	alias2model   map[string]string
	model2aliases map[string][]string
	exposeAliases bool
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
		cid string
		err error
	}
	ch := make(chan idErr)
	wg := sync.WaitGroup{}
	for cid, cl := range r.cmap {
		wg.Add(1)
		go func(id string, cl IOllamaClient) {
			defer wg.Done()
			err := cl.Heartbeat(ctx)
			ch <- idErr{cid: cid, err: err}
		}(cid, cl)
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
	for cid, v := range r.cmap {
		wg.Add(1)
		go func(cid string, cl IOllamaClient) {
			defer wg.Done()
			v, err := cl.List(ctx)
			if err != nil {
				log.WithField("connection_id", cid).WithError(err).Errorf("Failed to retrieve models.")
			}
			if v != nil {
				ids := r.cids2models[cid]
				v.Models = r.filterListToMapedModels(v.Models, ids...)
			}
			ch <- v
		}(cid, v)
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
	for cid, v := range r.cmap {
		wg.Add(1)
		go func(cid string, cl IOllamaClient) {
			defer wg.Done()
			v, err := cl.ListRunning(ctx)
			if err != nil {
				log.WithField("connection_id", cid).WithError(err).Errorf("Failed to retrieve running models.")
			}
			if v != nil {
				ids := r.cids2models[cid]
				v.Models = r.filterRunningListToMapedModels(v.Models, ids...)
			}
			ch <- &rsp{
				v: v,
				e: err,
			}
		}(cid, v)
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
	for cid, v := range r.cmap {
		wg.Add(1)
		go func(id string, cl IOllamaClient) {
			defer wg.Done()
			v, err := cl.Version(ctx)
			if err != nil {
				log.WithField("connection_id", cid).WithError(err).Errorf("Failed to retrieve version.")
			}
			ch <- v
		}(cid, v)
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
	if _, ok := r.modelCfg[model]; !ok {
		return fmt.Errorf("alias %s points to unknown model %s", alias, model)
	}
	if _, ok := r.modelCfg[alias]; ok {
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
	cid := r.modelCfg[modelID].ConnectionID
	cl := r.cmap[cid]
	if cl == nil {
		if modelID != "" && modelID != requested {
			return nil, requested, NewHttpErrorf(http.StatusNotFound, "gollamas router is missing a valid route to model %s (%s)", requested, modelID)
		}
		return nil, requested, NewHttpErrorf(http.StatusNotFound, "gollamas router is missing a valid route to model %s", requested)
	}

	return cl, modelID, nil
}

type ConnectionConfig struct {
	ConnectionID string
	Url          string
}

func reconcileConnectionsAndProxyConfigs(cc map[string]ConnectionConfig, pc map[string]ModelConfig) (map[string]ConnectionConfig, map[string]ModelConfig, error) {
	cconf := map[string]ConnectionConfig{}
	pconf := map[string]ModelConfig{}

	urls2ids := map[string][]string{}
	if cc == nil {
		return nil, nil, errors.New("missing connections config")
	}
	if pc == nil {
		return nil, nil, errors.New("missing models config")
	}
	if len(pc) == 0 {
		return nil, nil, errors.New("empty models config")
	}
	for k, v := range cc {
		if _, ok := cconf[k]; ok {
			return nil, nil, fmt.Errorf("duplicate connection id: %s", k)
		}
		if v.ConnectionID != "" && v.ConnectionID != k {
			return nil, nil, fmt.Errorf("connection id mismatch: %s != %s", k, v.ConnectionID)
		}
		if v.Url == "" {
			return nil, nil, fmt.Errorf("connection %s has an empty url", k)
		}
		_, err := url.Parse(v.ConnectionID)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid connection url: %s", k)
		}
		urls2ids[v.Url] = append(urls2ids[v.Url], k)
		cconf[k] = ConnectionConfig{
			ConnectionID: k,
			Url:          v.Url,
		}
	}
	for k, v := range pc {
		// if the connection id known
		if _, ok := cconf[v.ConnectionID]; ok {
			pconf[k] = ModelConfig{
				ConnectionID: v.ConnectionID,
			}
		} else {
			// it should be a url
			_, err := url.Parse(v.ConnectionID)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid connection id or url: %s", v.ConnectionID)
			}
			// if the connection url is already known by another connection id
			if cid, ok := urls2ids[v.ConnectionID]; ok {
				pconf[k] = ModelConfig{
					ConnectionID: cid[0],
				}
			} else {
				// we have a new connection
				cconf[v.ConnectionID] = ConnectionConfig{
					ConnectionID: v.ConnectionID,
					Url:          v.ConnectionID,
				}
				pconf[k] = ModelConfig{
					ConnectionID: v.ConnectionID,
				}
				urls2ids[v.ConnectionID] = append(urls2ids[v.ConnectionID], v.ConnectionID)
				cconf[k] = ConnectionConfig{
					ConnectionID: v.ConnectionID,
					Url:          v.ConnectionID,
				}
			}
		}
	}
	return cconf, pconf, nil
}

func initClients(cconf map[string]ConnectionConfig) (map[string]IOllamaClient, error) {
	if cconf == nil {
		return nil, errors.New("missing proxy config")
	}
	if len(cconf) == 0 {
		return nil, errors.New("empty proxy config map")
	}
	cmap := map[string]IOllamaClient{}
	for k, v := range cconf {
		remote, err := url.Parse(v.Url)
		if err != nil {
			return nil, err
		}
		client := api.NewClient(remote, http.DefaultClient)
		cmap[k] = client
	}

	return cmap, nil
}

func initRouterAliasOpts(aliases map[string]string) []RouterOption {
	return []RouterOption{&RouterOptions{
		Aliases: aliases,
	}}
}
