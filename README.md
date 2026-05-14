# Twitter Feed Cleaner (DELETER)

TUI app for automatically cleaning Twitter/X timeline from unwanted retweets.

## Features

- 🎨 Terminal UI with real-time logs
- 🔍 Filter retweets by keywords
- 📅 Date filter support
- 💾 Session persistence (14 days)
- 🎯 Interactive setup wizard

## Quick Start

```bash
# Build
go build -o deleter .

# Run (first time - setup wizard starts automatically)
./deleter
```

### Setup Process

1. Run `extract.js` in browser DevTools on x.com
2. Copy output to the bot
3. Enter `auth_token` from DevTools → Application → Cookies
4. Done! Session saved for 14 days

See [SETUP.md](SETUP.md) for details.

## How It Works

Uses internal Twitter GraphQL API to:
1. Fetch your timeline (`UserTweets`)
2. Find retweets matching keywords
3. Delete them via `DeleteRetweet` mutation

## Project Structure

```
deleter/
├── main.go          # TUI + setup wizard
├── internal/        # Core logic
│   ├── auth/        # Session management
│   ├── config/      # Config loading
│   └── twitter/     # API client
├── extract.js       # Browser data extractor
└── SETUP.md         # Detailed setup guide
```

## License

MIT. Use at your own risk.
