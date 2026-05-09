package main

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

type twitterX struct{}

func (twitterX) Name() string { return "x" }

func (twitterX) Match(u *url.URL) bool {
	h := strings.ToLower(u.Host)
	h = strings.TrimPrefix(h, "www.")
	h = strings.TrimPrefix(h, "mobile.")
	return h == "x.com" || h == "twitter.com"
}

func (twitterX) Resolve(ctx context.Context, u *url.URL) (*VideoSource, error) {
	return nil, errors.New("x/twitter resolver not implemented yet")
}
