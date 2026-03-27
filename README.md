# QraftWorx

Go CLI for AI-powered content automation. Uses Gemini as reasoning engine and Cerebro as persistent memory.

## Build

```bash
go build -o bin/qraft ./cmd/qraft
```

## Requirements

- Go 1.24+
- C compiler (CGO required for SQLite)
- ffmpeg (for media tools, optional for short tests)
