package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	out := flag.String("o", "", "output file (default: derived from post)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: schlonk [-o file] <post-url>\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	raw := flag.Arg(0)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		fatalf("invalid URL: %s", raw)
	}

	p, ok := matchPlatform(u)
	if !ok {
		fatalf("no platform handler for host %q", u.Host)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	src, err := p.Resolve(ctx, u)
	if err != nil {
		fatalf("%s: resolve: %v", p.Name(), err)
	}

	dst, err := download(ctx, src, *out)
	if err != nil {
		fatalf("download: %v", err)
	}
	fmt.Println(dst)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "schlonk: "+format+"\n", args...)
	os.Exit(1)
}
