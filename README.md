# schlonk

Small CLI to grab videos from Bluesky, X, and TikTok posts.

Yeah, I know yt-dlp exists. I just never remember where I put it. And the online "download this video" sites dump ads and viruses on you. So I vibe-coded my own.

## Use

    schlonk <post-url>

Saves the video in the current directory. Override the path with `-o`:

    schlonk -o clip.mp4 <post-url>

## Install

    go install github.com/crux/schlonk@latest

Single Go binary, lands in `~/go/bin/schlonk`. Or build from source:

    git clone https://github.com/crux/schlonk
    cd schlonk
    go build -o schlonk .

Cross-platform. Packages and a brew formula might come later.

## Supports

- Bluesky
- X / Twitter
- TikTok

Public posts only. Login-gated stuff (LinkedIn, Instagram) maybe later, we'll see.

## License

MIT.
