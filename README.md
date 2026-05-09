# schlonk

Small CLI to grab videos from social media posts.

Yeah, I know yt-dlp exists. I just never remember where I put it. And the online "download this video" sites dump ads and viruses on you. So I vibe-coded my own.

## Supports

- Bluesky
- X / Twitter
- TikTok

Public posts only. Login-gated stuff (LinkedIn, Instagram) maybe later, we'll see.

## Build

    go build -o schlonk .

Single Go binary, cross-platform. Packages and a brew formula might come later.

## Use

    schlonk <post-url>

Saves the video in the current directory. Override the path with `-o`:

    schlonk -o clip.mp4 <post-url>

## License

MIT.
