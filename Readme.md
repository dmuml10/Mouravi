# Go Telegram Bot 🤖

A simple Telegram bot written in Go that uses long polling to receive messages and respond to users.

## Features

- Uses Telegram Bot API (`getUpdates` + `sendMessage`)
- Long polling for real-time message handling
- Basic command support (`/start`)
- Echoes user messages
- Minimal dependencies (standard library only)

## How It Works

The bot continuously polls Telegram for updates using the `getUpdates` endpoint.  
When a message is received:

- `/start` → replies with a welcome message
- Any other text → echoes back the message

## Requirements

- Go 1.18+
- A Telegram Bot Token (from [@BotFather](https://t.me/BotFather))

## Setup

1. **Clone the repository**
   ```bash
   git clone <your-repo-url>
   cd <your-project>

export BOT_TOKEN=your_telegram_bot_token

go run main.go