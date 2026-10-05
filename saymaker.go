// Package saymaker generates images and video from Go on SayMaker
// (https://saymaker.ai).
//
// One API key runs the whole SayMaker shelf on your own credits: Veo 3.1,
// Kling 3.0, Seedance 2.0, MiniMax H3, Nano Banana 2, GPT Image 2.5,
// Seedream 5.0 and more. Create a key at https://saymaker.ai/settings/apikeys
// and pass it in Options.APIKey or set SAYMAKER_API_KEY.
//
//	url, err := saymaker.Generate(ctx, "a paper boat at sunrise", saymaker.Options{AspectRatio: "16:9"})
//	clip, err := saymaker.GenerateVideo(ctx, "a fox trotting through snow", saymaker.VideoOptions{Duration: 6})
//
// Standard library only.
package saymaker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// BaseURL is the service this package talks to. A variable so a test can point it elsewhere.
var BaseURL = "https://saymaker.ai"

// KeysURL is where an API key is created.
const KeysURL = "https://saymaker.ai/settings/apikeys"

// Default models. ModelImage is the cheapest text-to-image run, ModelEdit takes
// an input image, and ModelVideo is the video model a free account can run
// (480p or 768p, 4 to 15 seconds).
const (
	ModelImage = "saymaker-image-v1"
	ModelEdit  = "nano-banana-2-lite"
	ModelVideo = "minimax-h3-fast"
)

// Models maps a model id to the routing field the API expects next to it.
// Ids not listed are sent with "kie".
var Models = map[string]string{
	"saymaker-image-v1": "kie", "nano-banana-2-lite": "kie", "nano-banana-2": "poyo",
	"nano-banana-pro": "poyo", "gpt-image-2": "poyo", "gpt-image-2-5-flare": "kie",
	"seedream-5-lite": "poyo", "seedream-5-pro": "kie", "qwen-image-3-pro": "kie",
	"grok-imagine-image-2-0": "poyo",
	"minimax-h3-fast":        "vgenv", "minimax-h3": "poyo", "seedance-2": "kie",
	"seedance-2-fast": "kie", "seedance-2-5": "kie", "veo-3-1": "kie", "kling-3-0": "kie",
	"kling-3-0-turbo": "kie", "wan-3-0": "kie", "ltx-2-5-fast": "replicate",
}

// Errors worth branching on.
var (
	// ErrAuth: no API key, or the key is invalid or deleted.
	ErrAuth = errors.New("saymaker: API key missing or invalid")
	// ErrQuota: the account is out of credits.
	ErrQuota = errors.New("saymaker: insufficient credits")
	// ErrPlan: the model needs a paid plan or a credit pack.
	ErrPlan = errors.New("saymaker: this model needs a paid plan or a credit pack")
	// ErrBusy: another run on this account is still rendering. Free accounts run one at a time.
	ErrBusy = errors.New("saymaker: another run on this account is still rendering")
	// ErrRejected: the content filter or the model refused the prompt. Reword it.
	ErrRejected = errors.New("saymaker: the run failed; reword the prompt rather than retrying the same text")
)

// Options tunes an image call. The zero value is valid.
type Options struct {
	APIKey      string        // empty means SAYMAKER_API_KEY
	Model       string        // empty means ModelImage, or ModelEdit when ImageURL is set
	ImageURL    string        // a public http(s) url; makes the call an edit
	AspectRatio string        // "1:1", "16:9", "9:16", "4:3", "3:4"; empty keeps the default
	PollEvery   time.Duration // zero means five seconds
	Timeout     time.Duration // zero means 75 minutes (covers the free-account queue)
	HTTPClient  *http.Client  // nil means http.DefaultClient
}

// VideoOptions tunes a video call. The zero value is valid.
type VideoOptions struct {
	APIKey      string
	Model       string // empty means ModelVideo
	ImageURL    string // optional first frame; makes the call image-to-video
	Duration    int    // seconds, where the model offers a choice
	Resolution  string // e.g. "480p", "720p", "1080p"
	AspectRatio string // empty means "16:9"
	NoSound     bool
	PollEvery   time.Duration // zero means ten seconds
	Timeout     time.Duration // zero means 90 minutes
	HTTPClient  *http.Client
}

type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type result struct {
	ID          string   `json:"id"`
	Wall        bool     `json:"wall"`
	Status      string   `json:"status"`
	Images      []string `json:"images"`
	CleanImages []string `json:"cleanImages"`
	Videos      []string `json:"videos"`
}

// Generate creates one image (or edits Options.ImageURL) and returns its URL.
// For an edit, the prompt names the change and what must stay.
func Generate(ctx context.Context, prompt string, o Options) (string, error) {
	model, scene, size := o.Model, "text-to-image", o.AspectRatio
	opts := map[string]any{"resolution": "1K", "n": 1}
	if o.ImageURL != "" {
		if !isURL(o.ImageURL) {
			return "", errors.New("saymaker: ImageURL must be a public http(s) url")
		}
		scene = "image-to-image"
		opts["image_input"] = []string{o.ImageURL}
		if model == "" {
			model = ModelEdit
		}
		if size == "" {
			size = "auto"
		}
	}
	if model == "" {
		model = ModelImage
	}
	if size == "" {
		size = "1:1"
	}
	opts["size"] = size
	return run(ctx, o.APIKey, o.HTTPClient, o.PollEvery, o.Timeout, 5*time.Second, 75*time.Minute, "image", scene, model, prompt, opts)
}

// GenerateVideo creates one clip (or animates VideoOptions.ImageURL) and returns its URL.
// Video takes minutes. Veo 3.1, Kling 3.0 and Seedance 2.0 need a plan or a credit pack.
func GenerateVideo(ctx context.Context, prompt string, o VideoOptions) (string, error) {
	model, scene, ratio := o.Model, "text-to-video", o.AspectRatio
	if model == "" {
		model = ModelVideo
	}
	if ratio == "" {
		ratio = "16:9"
	}
	opts := map[string]any{"aspect_ratio": ratio, "sound": !o.NoSound}
	if o.ImageURL != "" {
		if !isURL(o.ImageURL) {
			return "", errors.New("saymaker: ImageURL must be a public http(s) url")
		}
		scene = "image-to-video"
		opts["image_input"] = []string{o.ImageURL}
	}
	if o.Duration > 0 {
		opts["duration"] = o.Duration
	}
	if o.Resolution != "" {
		opts["resolution"] = o.Resolution
	}
	return run(ctx, o.APIKey, o.HTTPClient, o.PollEvery, o.Timeout, 10*time.Second, 90*time.Minute, "video", scene, model, prompt, opts)
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func run(ctx context.Context, key string, client *http.Client, poll, timeout, defPoll, defTimeout time.Duration,
	media, scene, model, prompt string, opts map[string]any) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("saymaker: prompt is required")
	}
	if key == "" {
		key = os.Getenv("SAYMAKER_API_KEY")
	}
	if key == "" {
		return "", fmt.Errorf("%w: set SAYMAKER_API_KEY or Options.APIKey; create a key at %s", ErrAuth, KeysURL)
	}
	if client == nil {
		client = http.DefaultClient
	}
	if poll <= 0 {
		poll = defPoll
	}
	if timeout <= 0 {
		timeout = defTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	provider, ok := Models[model]
	if !ok {
		provider = "kie"
	}
	sub, err := post(ctx, client, key, "/api/ai/generate", map[string]any{
		"provider": provider, "mediaType": media, "model": model,
		"scene": scene, "prompt": prompt, "options": opts,
	})
	if err != nil {
		return "", err
	}
	// A signed-out request is HTTP 200 with code 0 — only `wall` tells it apart.
	if sub.Wall {
		return "", fmt.Errorf("%w: create a key at %s", ErrAuth, KeysURL)
	}
	if sub.ID == "" {
		return "", errors.New("saymaker: the service returned no task id")
	}
	// Free-account runs wait in a queue and only reach the model on the poll
	// that crosses the end of the wait, so polling is what starts the work.
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("saymaker: deadline passed; the run may still finish in your SayMaker history: %w", ctx.Err())
		case <-time.After(poll):
		}
		r, err := post(ctx, client, key, "/api/ai/query", map[string]any{"taskId": sub.ID})
		if err != nil {
			if errors.Is(err, ErrAuth) {
				return "", err
			}
			continue // a dropped poll is not a failed generation
		}
		for _, list := range [][]string{r.CleanImages, r.Images, r.Videos} {
			if len(list) > 0 {
				return list[0], nil
			}
		}
		if s := strings.ToLower(r.Status); s == "failed" || s == "error" {
			return "", ErrRejected
		}
	}
}

func post(ctx context.Context, client *http.Client, key, path string, payload any) (result, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", "saymaker-go/0.1.0")
	resp, err := client.Do(req)
	if err != nil {
		return result{}, err
	}
	defer resp.Body.Close()
	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return result{}, err
	}
	// Refusals answer HTTP 200 with code -1; reading the status code alone reports success.
	if env.Code != 0 {
		return result{}, classify(env.Message)
	}
	var r result
	_ = json.Unmarshal(env.Data, &r)
	return r, nil
}

func classify(msg string) error {
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "is rendering") || strings.Contains(low, "at once"):
		return fmt.Errorf("%w: %s", ErrBusy, msg)
	case strings.Contains(low, "api key") || strings.Contains(low, "no auth"):
		return fmt.Errorf("%w: %s", ErrAuth, msg)
	case strings.Contains(low, "insufficient credits"):
		return fmt.Errorf("%w: %s", ErrQuota, msg)
	case strings.Contains(low, "subscription") || strings.Contains(low, "plan") || strings.Contains(low, "sign in"):
		return fmt.Errorf("%w: %s", ErrPlan, msg)
	}
	return errors.New("saymaker: " + msg)
}
