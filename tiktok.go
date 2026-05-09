package main

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

type tiktok struct{}

func (tiktok) Name() string { return "tiktok" }

func (tiktok) Match(u *url.URL) bool {
	h := strings.ToLower(u.Host)
	h = strings.TrimPrefix(h, "www.")
	h = strings.TrimPrefix(h, "m.")
	return h == "tiktok.com" || h == "vm.tiktok.com" || h == "vt.tiktok.com"
}

func (tiktok) Resolve(ctx context.Context, u *url.URL) (*VideoSource, error) {
	return nil, errors.New("tiktok resolver not implemented yet")
}
