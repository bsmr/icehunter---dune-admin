.PHONY: build web go linux dev-server setup

build: web go

web:
	cd web && npm ci && npm run build

go:
	go build -o dune-admin .

linux:
	cd web && npm ci && npm run build
	GOOS=linux GOARCH=amd64 go build -o dune-admin-linux .

dev-server:
	go run .

setup:
	go run . -setup
