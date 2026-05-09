package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
)

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// httpClient is the shared client. It follows redirects by default and carries
// a cookie jar so platforms that hand out session cookies on the HTML fetch
// (TikTok in particular) can reuse them when fetching the signed video URL.
var httpClient = func() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}()

// download writes the video to userPath if non-empty, otherwise to a path
// derived from src.SuggestedBasename plus an extension chosen by the format.
// Returns the path actually written.
func download(ctx context.Context, src *VideoSource, userPath string) (string, error) {
	switch src.Kind {
	case "https":
		dst := userPath
		if dst == "" {
			dst = pickBasename(src) + ".mp4"
		}
		return dst, downloadHTTPS(ctx, src, dst)
	case "hls":
		return downloadHLS(ctx, src, userPath)
	default:
		return "", fmt.Errorf("unknown source kind %q", src.Kind)
	}
}

func pickBasename(src *VideoSource) string {
	if src.SuggestedBasename != "" {
		return src.SuggestedBasename
	}
	return "video"
}

func downloadHTTPS(ctx context.Context, src *VideoSource, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
	if err != nil {
		return err
	}
	applyHeaders(req, src.Headers)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("GET %s: %s", src.URL, resp.Status)
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}

func applyHeaders(req *http.Request, h http.Header) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", userAgent)
	}
	for k, vs := range h {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
}
