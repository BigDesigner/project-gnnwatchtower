package incident

import (
	"fmt"
	"gnnwatchtower/internal/netpolicy"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var alertQueue = make(chan func(), 128)
var alertMu sync.Mutex
var lastAlerts = make(map[string]time.Time)

func init() {
	for i := 0; i < 4; i++ {
		go func() {
			for job := range alertQueue {
				job()
			}
		}()
	}
}
func enqueueAlert(job func()) {
	select {
	case alertQueue <- job:
	default:
		log.Print("[Alert Engine] Notification queue full; incident remains recorded")
	}
}
func permitAlert(key string) bool {
	alertMu.Lock()
	defer alertMu.Unlock()
	now := time.Now()
	for k, t := range lastAlerts {
		if now.Sub(t) >= 3*time.Minute {
			delete(lastAlerts, k)
		}
	}
	if _, ok := lastAlerts[key]; ok {
		return false
	}
	if len(lastAlerts) >= 4096 {
		log.Print("[Alert Engine] Notification cooldown capacity reached")
		return false
	}
	lastAlerts[key] = now
	return true
}

type retryTransport struct{ base http.RoundTripper }

func (t retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || req.URL.User != nil {
		return nil, fmt.Errorf("notification destination requires HTTPS")
	}
	for attempt := 0; ; attempt++ {
		resp, err := t.base.RoundTrip(req)
		if err != nil || resp.StatusCode != 429 || attempt >= 2 || req.GetBody == nil {
			return resp, err
		}
		delay := time.Duration(1<<attempt) * time.Second
		if value := resp.Header.Get("Retry-After"); value != "" {
			if n, e := strconv.Atoi(value); e == nil {
				delay = time.Duration(n) * time.Second
			} else if at, e := http.ParseTime(value); e == nil {
				delay = time.Until(at)
			} else {
				return resp, nil
			}
		}
		if delay < time.Second {
			delay = time.Second
		}
		if delay > 30*time.Second {
			return resp, nil
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		}
		clone := req.Clone(req.Context())
		clone.Body, err = req.GetBody()
		if err != nil {
			return nil, err
		}
		req = clone
	}
}
func notificationClient() *http.Client {
	return &http.Client{Timeout: 25 * time.Second, Transport: retryTransport{netpolicy.Transport()}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
