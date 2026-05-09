package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const bskyXRPCBase = "https://public.api.bsky.app/xrpc"

type bluesky struct{}

func (bluesky) Name() string { return "bluesky" }

func (bluesky) Match(u *url.URL) bool {
	h := strings.ToLower(u.Host)
	return h == "bsky.app" || strings.HasSuffix(h, ".bsky.app")
}

func (bluesky) Resolve(ctx context.Context, u *url.URL) (*VideoSource, error) {
	handle, rkey, err := parseBlueskyPostURL(u)
	if err != nil {
		return nil, err
	}

	did := handle
	if !strings.HasPrefix(did, "did:") {
		did, err = bskyResolveHandle(ctx, handle)
		if err != nil {
			return nil, fmt.Errorf("resolve handle %q: %w", handle, err)
		}
	}

	atURI := fmt.Sprintf("at://%s/app.bsky.feed.post/%s", did, rkey)
	playlist, err := bskyGetVideoPlaylist(ctx, atURI)
	if err != nil {
		return nil, err
	}

	return &VideoSource{
		Kind:              "hls",
		URL:               playlist,
		SuggestedBasename: rkey,
	}, nil
}

// parseBlueskyPostURL expects /profile/<handle-or-did>/post/<rkey>.
func parseBlueskyPostURL(u *url.URL) (handle, rkey string, err error) {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "profile" || parts[2] != "post" {
		return "", "", fmt.Errorf("not a bluesky post URL: %s", u)
	}
	return parts[1], parts[3], nil
}

func bskyResolveHandle(ctx context.Context, handle string) (string, error) {
	var out struct {
		DID string `json:"did"`
	}
	err := bskyGetJSON(ctx, bskyXRPCBase+"/com.atproto.identity.resolveHandle?handle="+url.QueryEscape(handle), &out)
	if err != nil {
		return "", err
	}
	if out.DID == "" {
		return "", fmt.Errorf("empty DID in response")
	}
	return out.DID, nil
}

func bskyGetVideoPlaylist(ctx context.Context, atURI string) (string, error) {
	var out struct {
		Thread struct {
			Post struct {
				Embed struct {
					Type     string `json:"$type"`
					Playlist string `json:"playlist"`
				} `json:"embed"`
			} `json:"post"`
		} `json:"thread"`
	}
	err := bskyGetJSON(ctx, bskyXRPCBase+"/app.bsky.feed.getPostThread?uri="+url.QueryEscape(atURI), &out)
	if err != nil {
		return "", err
	}
	if out.Thread.Post.Embed.Type != "app.bsky.embed.video#view" {
		t := out.Thread.Post.Embed.Type
		if t == "" {
			return "", fmt.Errorf("post has no embed (no video)")
		}
		return "", fmt.Errorf("post embed is %s, not a video", t)
	}
	if out.Thread.Post.Embed.Playlist == "" {
		return "", fmt.Errorf("video embed has no playlist URL")
	}
	return out.Thread.Post.Embed.Playlist, nil
}

func bskyGetJSON(ctx context.Context, urlStr string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("GET %s: %s: %s", urlStr, resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(into)
}
