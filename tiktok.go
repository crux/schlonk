package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
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

// (?s) makes . match newlines. The script body is the embedded JSON blob.
var tiktokScriptRe = regexp.MustCompile(
	`(?s)<script[^>]*id="__UNIVERSAL_DATA_FOR_REHYDRATION__"[^>]*>(.+?)</script>`)

func (tiktok) Resolve(ctx context.Context, u *url.URL) (*VideoSource, error) {
	body, finalURL, err := tiktokFetchHTML(ctx, u.String())
	if err != nil {
		return nil, err
	}

	m := tiktokScriptRe.FindSubmatch(body)
	if m == nil {
		return nil, fmt.Errorf("could not find rehydration data on page (login wall, region block, or page format changed?)")
	}

	var blob struct {
		Scope struct {
			Detail struct {
				ItemInfo struct {
					ItemStruct struct {
						ID    string `json:"id"`
						Video struct {
							PlayAddr     string `json:"playAddr"`
							DownloadAddr string `json:"downloadAddr"`
							BitrateInfo  []struct {
								Bitrate  int `json:"Bitrate"`
								PlayAddr struct {
									URLList []string `json:"UrlList"`
								} `json:"PlayAddr"`
							} `json:"bitrateInfo"`
						} `json:"video"`
					} `json:"itemStruct"`
				} `json:"itemInfo"`
			} `json:"webapp.video-detail"`
		} `json:"__DEFAULT_SCOPE__"`
	}
	if err := json.Unmarshal(m[1], &blob); err != nil {
		return nil, fmt.Errorf("parse rehydration JSON: %w", err)
	}

	item := blob.Scope.Detail.ItemInfo.ItemStruct
	video := item.Video

	bestURL := ""
	bestBitrate := -1
	for _, b := range video.BitrateInfo {
		if b.Bitrate > bestBitrate && len(b.PlayAddr.URLList) > 0 {
			bestURL = b.PlayAddr.URLList[0]
			bestBitrate = b.Bitrate
		}
	}
	if bestURL == "" {
		// Fallback for slideshow / older shapes where bitrateInfo is missing.
		bestURL = video.PlayAddr
	}
	if bestURL == "" {
		return nil, fmt.Errorf("no video stream URL found in page (might be a slideshow post)")
	}

	headers := http.Header{}
	headers.Set("Referer", "https://www.tiktok.com/")

	id := item.ID
	if id == "" {
		id = tiktokIDFromPath(finalURL.Path)
	}
	if id == "" {
		id = "tiktok"
	}

	return &VideoSource{
		Kind:              "https",
		URL:               bestURL,
		Headers:           headers,
		SuggestedBasename: id,
	}, nil
}

func tiktokIDFromPath(p string) string {
	parts := strings.Split(strings.Trim(p, "/"), "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "video" {
			return parts[i+1]
		}
	}
	return ""
}

func tiktokFetchHTML(ctx context.Context, urlStr string) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, nil, fmt.Errorf("GET %s: %s", urlStr, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return body, resp.Request.URL, nil
}
