package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
)

type hlsVariant struct {
	bandwidth int
	uri       string
}

// downloadHLS fetches src.URL as an HLS playlist (master or media), picks the
// highest-bandwidth variant if it's a master, then downloads the init segment
// (when EXT-X-MAP is present) followed by every media segment, concatenated.
// If userPath is empty, the output is named src.SuggestedBasename + ".mp4".
// Returns the path actually written.
//
// The bytes may be MPEG-TS (no init segment) named .mp4 — most players handle
// this; if QuickTime trips on it later, swap in a real TS→MP4 remux step here.
func downloadHLS(ctx context.Context, src *VideoSource, userPath string) (string, error) {
	masterText, masterBase, err := hlsFetch(ctx, src.URL, src.Headers)
	if err != nil {
		return "", fmt.Errorf("fetch playlist: %w", err)
	}

	variants := hlsParseMaster(masterText)
	mediaURL := src.URL
	mediaBase := masterBase
	mediaText := masterText
	if len(variants) > 0 {
		sort.Slice(variants, func(i, j int) bool { return variants[i].bandwidth > variants[j].bandwidth })
		mediaURL, err = hlsResolveURL(masterBase, variants[0].uri)
		if err != nil {
			return "", err
		}
		mediaText, mediaBase, err = hlsFetch(ctx, mediaURL, src.Headers)
		if err != nil {
			return "", fmt.Errorf("fetch media playlist: %w", err)
		}
	}

	initURI, segments := hlsParseMedia(mediaText)
	if len(segments) == 0 {
		return "", fmt.Errorf("media playlist has no segments")
	}

	dst := userPath
	if dst == "" {
		dst = pickBasename(src) + ".mp4"
	}

	f, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if initURI != "" {
		u, err := hlsResolveURL(mediaBase, initURI)
		if err != nil {
			return "", err
		}
		if err := hlsAppend(ctx, f, u, src.Headers); err != nil {
			return "", fmt.Errorf("init segment: %w", err)
		}
	}

	for i, seg := range segments {
		u, err := hlsResolveURL(mediaBase, seg)
		if err != nil {
			return "", err
		}
		if err := hlsAppend(ctx, f, u, src.Headers); err != nil {
			return "", fmt.Errorf("segment %d/%d: %w", i+1, len(segments), err)
		}
		fmt.Fprintf(os.Stderr, "\rsegments: %d/%d", i+1, len(segments))
	}
	fmt.Fprintln(os.Stderr)
	return dst, nil
}

func hlsFetch(ctx context.Context, urlStr string, headers http.Header) (text string, finalURL *url.URL, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", nil, err
	}
	applyHeaders(req, headers)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", nil, fmt.Errorf("GET %s: %s", urlStr, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}
	return string(body), resp.Request.URL, nil
}

func hlsAppend(ctx context.Context, w io.Writer, urlStr string, headers http.Header) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return err
	}
	applyHeaders(req, headers)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("GET %s: %s", urlStr, resp.Status)
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

func hlsParseMaster(text string) []hlsVariant {
	var out []hlsVariant
	sc := bufio.NewScanner(strings.NewReader(text))
	var pending *hlsVariant
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			v := &hlsVariant{}
			attrs := parseAttrList(strings.TrimPrefix(line, "#EXT-X-STREAM-INF:"))
			if bw, ok := attrs["BANDWIDTH"]; ok {
				v.bandwidth, _ = strconv.Atoi(bw)
			}
			pending = v
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if pending != nil {
			pending.uri = line
			out = append(out, *pending)
			pending = nil
		}
	}
	return out
}

func hlsParseMedia(text string) (initURI string, segments []string) {
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-MAP:") {
			attrs := parseAttrList(strings.TrimPrefix(line, "#EXT-X-MAP:"))
			initURI = attrs["URI"]
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		segments = append(segments, line)
	}
	return
}

// parseAttrList parses an HLS attribute list like KEY=VALUE,KEY="quoted",N=12.
func parseAttrList(s string) map[string]string {
	out := map[string]string{}
	i := 0
	for i < len(s) {
		eq := strings.IndexByte(s[i:], '=')
		if eq < 0 {
			break
		}
		key := strings.TrimSpace(s[i : i+eq])
		i += eq + 1
		var val string
		if i < len(s) && s[i] == '"' {
			i++
			end := strings.IndexByte(s[i:], '"')
			if end < 0 {
				break
			}
			val = s[i : i+end]
			i += end + 1
			if i < len(s) && s[i] == ',' {
				i++
			}
		} else {
			comma := strings.IndexByte(s[i:], ',')
			if comma < 0 {
				val = s[i:]
				i = len(s)
			} else {
				val = s[i : i+comma]
				i += comma + 1
			}
		}
		out[key] = val
	}
	return out
}

func hlsResolveURL(base *url.URL, ref string) (string, error) {
	refU, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(refU).String(), nil
}
