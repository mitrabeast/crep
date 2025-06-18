# crep - Custom Registry Publisher

A lightweight Go tool that uses Dagger.io to build and push containers to custom registries.

## Prerequisites

- Go 1.21+
- Docker daemon
- Registry access

## Setup

```bash
go mod tidy
```

## Environment Variables

```bash
export REG_ADDR="your-registry.com"    # required
export REG_USER="username"             # optional
export REG_PASS="password"             # optional
export IMG_NAME="my-app"               # required
export IMG_TAG="v1.0.0"                # optional (defaults to "latest")
```

## Usage

```bash
go run main.go
```

## Examples

Push to Docker Hub with auth:
```bash
export REG_ADDR="docker.io"
export REG_USER="myuser"
export REG_PASS="mypass"
export IMG_NAME="hello-python"
go run main.go
```

Push to local registry without auth:
```bash
export REG_ADDR="localhost:5000"
export IMG_NAME="hello-python"
export IMG_TAG="dev"
go run main.go
```

## Features

- Flexible image naming and tagging
- Optional registry authentication
- Clean error handling
- Built with Dagger.io for reliable builds