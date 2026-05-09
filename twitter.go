package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
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

var twitterStatusRe = regexp.MustCompile(`/status/(\d+)`)

func (twitterX) Resolve(ctx context.Context, u *url.URL) (*VideoSource, error) {
	m := twitterStatusRe.FindStringSubmatch(u.Path)
	if m == nil {
		return nil, fmt.Errorf("not a tweet URL: %s", u)
	}
	id := m[1]

	apiU, _ := url.Parse("https://cdn.syndication.twimg.com/tweet-result")
	q := apiU.Query()
	q.Set("id", id)
	q.Set("token", twitterSyndicationToken(id))
	q.Set("lang", "en")
	apiU.RawQuery = q.Encode()

	var resp struct {
		MediaDetails []struct {
			Type      string `json:"type"`
			VideoInfo struct {
				Variants []struct {
					Bitrate     int    `json:"bitrate"`
					ContentType string `json:"content_type"`
					URL         string `json:"url"`
				} `json:"variants"`
			} `json:"video_info"`
		} `json:"mediaDetails"`
	}
	if err := twitterGetJSON(ctx, apiU.String(), &resp); err != nil {
		return nil, err
	}

	var bestURL string
	bestBitrate := -1
	for _, md := range resp.MediaDetails {
		for _, v := range md.VideoInfo.Variants {
			if v.ContentType == "video/mp4" && v.Bitrate > bestBitrate {
				bestURL = v.URL
				bestBitrate = v.Bitrate
			}
		}
	}
	if bestURL == "" {
		return nil, fmt.Errorf("tweet has no mp4 video variants")
	}

	return &VideoSource{
		Kind:              "https",
		URL:               bestURL,
		SuggestedBasename: id,
	}, nil
}

// twitterSyndicationToken implements the JS expression:
//
//	((Number(id) / 1e15) * Math.PI).toString(36).replace(/(0+|\.)/g, '')
//
// That's the scheme the embed iframe loader uses, accepted by
// cdn.syndication.twimg.com/tweet-result.
func twitterSyndicationToken(id string) string {
	n, err := strconv.ParseFloat(id, 64)
	if err != nil {
		return ""
	}
	f := (n / 1e15) * math.Pi
	s := jsFloatToBase36(f)
	s = strings.ReplaceAll(s, "0", "")
	s = strings.ReplaceAll(s, ".", "")
	return s
}

// jsFloatToBase36 emulates JS Number.prototype.toString(36) for positive
// finite doubles. It emits the shortest sequence of base-36 digits (with
// an optional ".") that round-trips to f under parseBase36Float.
func jsFloatToBase36(f float64) string {
	if f == 0 {
		return "0"
	}
	intPart := math.Floor(f)
	intStr := strconv.FormatUint(uint64(intPart), 36)
	if f == intPart {
		return intStr
	}

	rem := f - intPart
	digits := make([]byte, 0, 20)
	for i := 0; i < 25; i++ {
		rem *= 36
		d := byte(math.Floor(rem))
		if d > 35 {
			d = 35
		}
		digits = append(digits, base36Char(d))
		rem -= float64(d)

		candidate := intStr + "." + string(digits)
		if parseBase36Float(candidate) == f {
			return candidate
		}
	}
	return intStr + "." + string(digits)
}

func base36Char(d byte) byte {
	if d < 10 {
		return '0' + d
	}
	return 'a' + d - 10
}

func base36Value(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'z':
		return c - 'a' + 10
	case c >= 'A' && c <= 'Z':
		return c - 'A' + 10
	}
	return 0
}

func parseBase36Float(s string) float64 {
	dot := strings.IndexByte(s, '.')
	intStr, fracStr := s, ""
	if dot >= 0 {
		intStr, fracStr = s[:dot], s[dot+1:]
	}
	intVal, _ := strconv.ParseUint(intStr, 36, 64)
	result := float64(intVal)
	mult := 1.0 / 36
	for i := 0; i < len(fracStr); i++ {
		result += float64(base36Value(fracStr[i])) * mult
		mult /= 36
	}
	return result
}

func twitterGetJSON(ctx context.Context, urlStr string, into any) error {
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
