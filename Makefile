.PHONY: run dev fmt fmt-go fmt-html css build

run:
	go run .

dev:
	go tool air

fmt: fmt-go fmt-html

fmt-go:
	go fmt ./...

fmt-html:
	pnpm prettier --write "templates/**/*.html"

css:
	pnpm tailwindcss -i ./static/css/input.css -o ./static/css/output.css

build: css
	go build -o ./tmp/main .
