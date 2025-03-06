package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ollama/ollama/api"
)

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

func abortGinError(c *gin.Context, err error) {
	var httpErr *HttpError
	if errors.As(err, &httpErr) {
		c.AbortWithStatusJSON(httpErr.StatusCode(), gin.H{"error": httpErr.Error()})
		return
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
