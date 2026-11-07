package ai

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryDelay is the pause before the first retry; it doubles each time. Tests shorten it.
var RetryDelay = 800 * time.Millisecond

// Do sends the request built by build, retrying when the service is busy
// (429, 500, 502, 503, 504, 529) or the connection fails. It returns the first
// response that is not a transient failure; the caller closes its body.
func Do(ctx context.Context, client *http.Client, build func() (*http.Request, error)) (*http.Response, error) {
	const attempts = 3
	delay := RetryDelay
	var lastErr error
	var lastResp *http.Response
	for i := 0; i < attempts; i++ {
		lastResp = nil
		req, err := build()
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req.WithContext(ctx))
		switch {
		case err == nil && !transient(resp.StatusCode):
			return resp, nil
		case err == nil:
			lastErr = fmt.Errorf("the service is busy (HTTP %d)", resp.StatusCode)
			if i == attempts-1 {
				return resp, nil // let the caller report the final status
			}
			lastResp = resp
			resp.Body.Close()
		default:
			lastErr = err
			if ctx.Err() != nil {
				return nil, err
			}
		}
		wait := delay
		if ra := retryAfter(lastResp); ra > wait {
			wait = ra
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		delay *= 2
	}
	return nil, lastErr
}

func transient(code int) bool {
	switch code {
	case 429, 500, 502, 503, 504, 529:
		return true
	}
	return false
}

// StatusError explains an HTTP failure in words a person can act on, without
// quoting the response body (which can echo the prompt).
func StatusError(label string, code int) error {
	switch code {
	case 400:
		return fmt.Errorf("%s rejected the request (HTTP 400): the model name may be wrong. In /models use  model N NAME", label)
	case 401, 403:
		return fmt.Errorf("%s rejected the API key (HTTP %d): check it with  key N  in /models", label, code)
	case 404:
		return fmt.Errorf("%s does not know this model (HTTP 404): it may have been retired. In /models use  model N NAME", label)
	case 429:
		return fmt.Errorf("%s says you are over your rate or quota limit (HTTP 429)", label)
	}
	return fmt.Errorf("%s answered HTTP %d", label, code)
}

// retryAfter reads the server's own advice on how long to wait (seconds), capped
// so a bad value cannot stall the terminal.
func retryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After")))
	if err != nil || n <= 0 {
		return 0
	}
	return min(time.Duration(n)*time.Second, 30*time.Second)
}
