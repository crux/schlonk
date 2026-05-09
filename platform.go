package main

import (
	"context"
	"net/http"
	"net/url"
)

// VideoSource describes how to fetch the video for a resolved post.
type VideoSource struct {
	// Kind tells the downloader how to handle URL: "https" for a single direct
	// download, "hls" for an m3u8 playlist that must be assembled from segments.
	Kind string

	URL               string
	Headers           http.Header // request headers required by the CDN (Referer, UA, cookies, ...)
	SuggestedBasename string      // filename without extension; downloader picks the extension based on actual format
}

// Platform handles resolving one social-media site.
type Platform interface {
	Name() string
	Match(u *url.URL) bool
	Resolve(ctx context.Context, u *url.URL) (*VideoSource, error)
}

var platforms = []Platform{
	bluesky{},
	twitterX{},
	tiktok{},
}

func matchPlatform(u *url.URL) (Platform, bool) {
	for _, p := range platforms {
		if p.Match(u) {
			return p, true
		}
	}
	return nil, false
}
