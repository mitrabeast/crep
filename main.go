package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"dagger.io/dagger"
)

type Registry struct {
	Addr string
	User string
	Pass string
}

func NewRegistry() *Registry {
	addr := os.Getenv("REG_ADDR")
	if addr == "" {
		log.Fatal("registry address is required")
	}
	addr = strings.TrimSuffix(addr, "/")
	return &Registry{
		Addr: addr,
		User: os.Getenv("REG_USER"),
		Pass: os.Getenv("REG_PASS"),
	}
}

func (r *Registry) HasCreds() bool {
	return r.User != "" && r.Pass != ""
}

type Image struct {
	Name string
	Tag  string
}

func NewImage() *Image {
	name := os.Getenv("IMG_NAME")
	if name == "" {
		log.Fatal("image name is required")
	}
	tag := os.Getenv("IMG_TAG")
	if tag == "" {
		tag = "latest"
	}
	return &Image{
		Name: name,
		Tag:  tag,
	}
}

func (i *Image) Ref() string {
	return fmt.Sprintf("%s:%s", i.Name, i.Tag)
}

func (i *Image) FullRef(addr string) string {
	return fmt.Sprintf("%s/%s", addr, i.Ref())
}

func main() {
	reg := NewRegistry()
	img := NewImage()
	ctx := context.Background()

	c, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer c.Close()

	ref := img.FullRef(reg.Addr)
	log.Printf("Pushing to %s...", ref)

	container := c.Container().
		From("python:3.11-slim").
		WithNewFile("/app/hello.py", dagger.ContainerWithNewFileOpts{
			Contents:    "#!/usr/bin/env python3\nprint('Hello World from Dagger!')",
			Permissions: 0755,
		}).
		WithWorkdir("/app").
		WithEntrypoint([]string{"python3", "hello.py"})

	if reg.HasCreds() {
		container = container.WithRegistryAuth(reg.Addr, reg.User, c.SetSecret("reg-pass", reg.Pass))
	}

	pushed, err := container.Publish(ctx, ref)
	if err != nil {
		log.Fatalf("Failed to push: %v", err)
	}

	log.Printf("Pushed: %s", pushed)
}
