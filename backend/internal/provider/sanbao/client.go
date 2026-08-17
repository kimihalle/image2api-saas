package sanbao

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultBaseURL = "https://sanbaobeauty.com"

var (
	ErrAuth              = errors.New("sanbao api key invalid")
	ErrQuotaExhausted    = errors.New("sanbao credits exhausted")
	ErrTemporaryUpstream = errors.New("sanbao temporary upstream error")
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 5 * time.Minute}}
}

type Account struct {
	ID      any     `json:"id"`
	Email   string  `json:"email"`
	Name    string  `json:"name"`
	Credits float64 `json:"credits"`
	Balance float64 `json:"balance"`
}

type Model struct {
	ID                                string             `json:"id"`
	Name                              string             `json:"name"`
	Description                       string             `json:"description"`
	Billing                           string             `json:"billing"`
	Resolutions                       []string           `json:"resolutions"`
	Ratios                            []string           `json:"ratios"`
	Durations                         []int              `json:"durations"`
	DefaultResolution                 string             `json:"default_resolution"`
	DefaultDurationSeconds            int                `json:"default_duration_seconds"`
	DefaultRatio                      string             `json:"default_ratio"`
	MaxPromptLength                   int                `json:"max_prompt_length"`
	MinImages                         int                `json:"min_images"`
	MaxImages                         int                `json:"max_images"`
	MaxVideos                         int                `json:"max_videos"`
	MaxAudios                         int                `json:"max_audios"`
	MaxMediaFiles                     int                `json:"max_media_files"`
	MaxImageSizeMB                    int                `json:"max_image_size_mb"`
	MaxVideoSizeMB                    int                `json:"max_video_size_mb"`
	MaxAudioSizeMB                    int                `json:"max_audio_size_mb"`
	SupportsRealPerson                bool               `json:"supports_real_person"`
	ConcurrencyOptions                []int              `json:"concurrency_options"`
	Enabled                           bool               `json:"enabled"`
	PricePerSecondByResolutionCredits map[string]float64 `json:"price_per_second_by_resolution_credits"`
	PriceByResolutionCredits          map[string]float64 `json:"price_by_resolution_credits"`
}

type Media struct {
	Tag string `json:"tag,omitempty"`
	URL string `json:"url"`
}

type CreateRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	Ratio       string  `json:"ratio,omitempty"`
	Resolution  string  `json:"resolution,omitempty"`
	Duration    int     `json:"duration,omitempty"`
	Concurrency int     `json:"concurrency,omitempty"`
	Reference   string  `json:"reference,omitempty"`
	Images      []Media `json:"images,omitempty"`
	Videos      []Media `json:"videos,omitempty"`
	Audios      []Media `json:"audios,omitempty"`
}

type Task struct {
	ID            string  `json:"id"`
	Status        string  `json:"status"`
	Progress      int     `json:"progress"`
	VideoURL      string  `json:"video_url"`
	DownloadURL   string  `json:"download_url"`
	LinkExpiresIn int     `json:"link_expires_in"`
	Cost          float64 `json:"cost"`
	Error         any     `json:"error"`
}

func (c *Client) Me(ctx context.Context, key string) (*Account, error) {
	var out Account
	if err := c.doJSON(ctx, http.MethodGet, "/openapi/v1/me", key, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Models(ctx context.Context, key string) ([]Model, error) {
	var raw json.RawMessage
	if err := c.doJSON(ctx, http.MethodGet, "/openapi/v1/models", key, nil, &raw); err != nil {
		return nil, err
	}
	return decodeModels(raw)
}

func decodeModels(raw json.RawMessage) ([]Model, error) {
	var direct []Model
	if json.Unmarshal(raw, &direct) == nil && direct != nil {
		return direct, nil
	}
	var wrapped struct {
		Data   []Model `json:"data"`
		Models []Model `json:"models"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, err
	}
	if wrapped.Data != nil {
		return wrapped.Data, nil
	}
	return wrapped.Models, nil
}

func (c *Client) Upload(ctx context.Context, key, kind, fileName, contentType string, body []byte) (string, error) {
	path := "/openapi/v1/uploads/raw?kind=" + url.QueryEscape(kind) + "&fileName=" + url.QueryEscape(fileName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	c.auth(req, key)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTemporaryUpstream, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err := statusError(resp.StatusCode, b); err != nil {
		return "", err
	}
	var out struct {
		URL  string `json:"url"`
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	if out.URL == "" {
		out.URL = out.Data.URL
	}
	if out.URL == "" {
		return "", errors.New("sanbao upload returned no url")
	}
	return out.URL, nil
}

func (c *Client) CreateVideo(ctx context.Context, key string, in CreateRequest) (*Task, error) {
	var out Task
	if err := c.doJSON(ctx, http.MethodPost, "/openapi/v1/videos", key, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Video(ctx context.Context, key, id string) (*Task, error) {
	var out Task
	if err := c.doJSON(ctx, http.MethodGet, "/openapi/v1/videos/"+url.PathEscape(id), key, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Download(ctx context.Context, rawURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrTemporaryUpstream, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if err := statusError(resp.StatusCode, b); err != nil {
		return nil, "", err
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "video/mp4"
	}
	return b, ct, nil
}

func (c *Client) doJSON(ctx context.Context, method, path, key string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	c.auth(req, key)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTemporaryUpstream, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err := statusError(resp.StatusCode, b); err != nil {
		return err
	}
	var wrapped struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(b, &wrapped); err == nil && len(wrapped.Data) > 0 && string(wrapped.Data) != "null" {
		if err := json.Unmarshal(wrapped.Data, out); err == nil {
			return nil
		}
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("invalid sanbao response: %w", err)
	}
	return nil
}

func (c *Client) auth(req *http.Request, key string) {
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(key))
	req.Header.Set("Accept", "application/json")
}

func statusError(code int, body []byte) error {
	if code >= 200 && code < 300 {
		return nil
	}
	msg := strings.TrimSpace(string(body))
	if len(msg) > 320 {
		msg = msg[:320]
	}
	switch code {
	case 401, 403:
		return fmt.Errorf("%w: %s", ErrAuth, msg)
	case 402:
		return fmt.Errorf("%w: %s", ErrQuotaExhausted, msg)
	case 429, 500, 502, 503, 504:
		return fmt.Errorf("%w: http %d %s", ErrTemporaryUpstream, code, msg)
	default:
		return fmt.Errorf("sanbao http %d: %s", code, msg)
	}
}
