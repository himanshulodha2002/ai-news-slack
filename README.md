# AI News Slack Bot

Posts new Latent.Space AI News updates into a Slack channel without needing your own server.

This version is written in Go.

Project layout:

- `cmd/ai-news-slack` contains the executable entrypoint
- `internal/app` contains the sync flow
- `internal/source` handles feed and article parsing
- `internal/slack` handles Slack posting
- `internal/config`, `internal/state`, and `internal/httpx` keep support code isolated

## Best Free Setup

Use GitHub Actions. GitHub runs the bot for you every 12 hours, so you do not need a VPS, Render app, or always-on laptop.

The workflow in [`.github/workflows/post-ai-news.yml`](/Users/himanshulodha/personal/ai-news-slack/.github/workflows/post-ai-news.yml) does this:

- runs every 12 hours
- fetches the latest AI News entries
- posts only new items to Slack
- saves `data/state.json` so the same post is not sent twice

## Slack Setup

1. Create a Slack app at `api.slack.com/apps`.
2. Add the bot scope `chat:write`.
3. Install the app to your workspace.
4. Invite the bot to your target channel.
5. Copy the channel ID from Slack.

You only need these two values:

- `SLACK_BOT_TOKEN`
- `SLACK_CHANNEL_ID`

## GitHub Setup

1. Push this project to a GitHub repo.
2. Open the repo settings.
3. Add these Actions secrets:
   - `SLACK_BOT_TOKEN`
   - `SLACK_CHANNEL_ID`
4. Optionally add these Actions variables if you want to override defaults:
   - `LATENT_SPACE_AINEWS_URL`
   - `MAX_POSTS_PER_RUN`
5. Enable GitHub Actions for the repo.
6. Run the workflow once manually from the Actions tab.

Defaults:

- `LATENT_SPACE_AINEWS_URL=https://www.latent.space/s/ainews`
- `MAX_POSTS_PER_RUN=1`
- `STATE_FILE=./data/state.json`

## Local Run

You only need this if you want to test before pushing:

```bash
go run ./cmd/ai-news-slack
```

## How The Bot Reads AI News

The bot:

- tries the main Latent.Space feed first
- filters for AI News posts
- falls back to the AI News section page if needed
- opens each article page to capture the real title and subtitle
- posts the root message, then the AI Twitter Recap as a Slack thread
- deduplicates by URL and publish timestamp

## Notes

GitHub scheduled workflows are not guaranteed to fire at the exact minute, but for a source that posts daily, a 12-hour schedule is a good fit.
