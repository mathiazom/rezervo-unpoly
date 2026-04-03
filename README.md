# 🏄 rezervo-unpoly

Server-rendered frontend for [rezervo](https://github.com/mathiazom/rezervo) using Go and [Unpoly](https://unpoly.com).

## 🧰 Prerequisites

- Go 1.23+
- Docker (for container-based dev)
- A running rezervo backend and FusionAuth instance

## 🏗️ Setup

```bash
cp .env.example .env
```

Fill in `.env`:

| Variable | Description |
|---|---|
| `FUSIONAUTH_URL` | Public FusionAuth URL (used for browser redirects) |
| `FUSIONAUTH_INTERNAL_URL` | Internal FusionAuth URL for server-to-server calls (optional, defaults to `FUSIONAUTH_URL`) |
| `FUSIONAUTH_CLIENT_ID` | OAuth2 client ID |
| `APP_URL` | Public URL of this app (determines secure cookie flag) |
| `REZERVO_API_URL` | rezervo backend URL |
| `SECRET_KEY` | 32-byte AES key as 64 hex chars — generate with `openssl rand -hex 32` |
| `PORT` | Server port (default: `3000`) |

## 🚀 Run locally

```bash
make run
```

With hot reload ([air](https://github.com/air-verse/air) watches Go files and templates):

```bash
make dev
```

Or with Docker:

```bash
docker compose up --build
```

Visit http://localhost:3000.

## 🧹 Formatting

Go:

```bash
make fmt
```

HTML templates ([prettier](https://prettier.io)):

```bash
npx prettier --write "templates/**/*.html"
```
