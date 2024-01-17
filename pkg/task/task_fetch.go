package task

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/samber/lo"
)

var validHttpMethods = map[string]bool{
	"GET":     true,
	"HEAD":    true,
	"POST":    true,
	"PUT":     true,
	"DELETE":  true,
	"CONNECT": true,
	"OPTIONS": true,
	"TRACE":   true,
	"PATCH":   true,
}

type TaskFetch struct {
	Url    string `json:"url"`
	Method string `json:"method"`
	Body   string `json:"body"`
}

func (h *TaskFetch) UnmarshalInline(text string) (err error) {
	if before, after, found := strings.Cut(text, " "); found {
		h.Method = before
		if !validHttpMethods[h.Method] {
			return fmt.Errorf("invalid HTTP method: %s", h.Method)
		}
		text = after
	}
	if before, after, found := strings.Cut(text, " "); found {
		text = before
		h.Body = after
	}
	h.Url = text
	return
}

func (h *TaskFetch) Launch(ctx Controller) <-chan error {
	return lo.Async(func() error {
		var requestBody io.Reader
		if h.Body != "" {
			requestBody = strings.NewReader(h.Body)
		}
		req, err := http.NewRequestWithContext(ctx, h.Method, h.Url, requestBody)
		if err != nil {
			return NonRetriable(err)
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}

		ctx.Whiteboard().Set("status", res.StatusCode)
		ctx.Logger().Printf("Status code: %d", res.StatusCode)
		defer res.Body.Close()

		if res.StatusCode != 200 {
			return err
		}

		body, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}

		var result any
		err = json.Unmarshal(body, &result)
		if err != nil {
			return err
		}
		ctx.Logger().Printf("Body: %v", result)
		ctx.Whiteboard().Set("body", result)
		return nil
	})
}

func init() {
	Registry.Define("fetch", TaskFetch{})
}
