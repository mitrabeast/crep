package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"dagger.io/dagger"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
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

func (r *Registry) Encode() string {
	authBytes, _ := json.Marshal(registry.AuthConfig{
		Username:      r.User,
		Password:      r.Pass,
		ServerAddress: r.Addr,
	})
	return base64.StdEncoding.EncodeToString(authBytes)
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

func (i *Image) String() string {
	return fmt.Sprintf("%s:%s", i.Name, i.Tag)
}

func (i *Image) FullRef(registryAddr string) string {
	return fmt.Sprintf("%s/%s", registryAddr, i.String())
}

func useDagger() bool {
	return strings.ToLower(os.Getenv("USE_DAGGER")) != "false"
}

func pushWithDagger(
	ctx context.Context,
	client *dagger.Client,
	container *dagger.Container,
	reg *Registry,
	img *Image,
) error {
	ref := img.FullRef(reg.Addr)

	if reg.HasCreds() {
		container = container.WithRegistryAuth(reg.Addr, reg.User, client.SetSecret("reg-pass", reg.Pass))
	}

	pushed, err := container.Publish(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to push: %v", err)
	}

	log.Printf("Pushed with Dagger: %s", pushed)
	return nil
}

func pushWithDocker(ctx context.Context, container *dagger.Container, reg *Registry, img *Image) error {
	log.Println("Connecting to Docker daemon...")
	docker, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create docker client: %v", err)
	}
	defer docker.Close()

	log.Println("Exporting image as tarball...")
	path := fmt.Sprintf("/tmp/dagger-push-%s.tar", img.String())
	if ok, err := container.Export(ctx, path, dagger.ContainerExportOpts{
		ForcedCompression: dagger.Zstd,
	}); !ok {
		return fmt.Errorf("failed to export container to %s", path)
	} else if err != nil {
		return fmt.Errorf("failed to export container: %v", err)
	}
	defer os.RemoveAll(path)

	log.Println("Reading tarball...")
	tarball, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to read tarball: %v", err)
	}
	defer tarball.Close()

	log.Println("Loading image into Docker daemon...")
	loadResp, err := docker.ImageLoad(ctx, tarball)
	if err != nil {
		return fmt.Errorf("failed to load image: %v", err)
	}
	defer loadResp.Body.Close()

	loadOutput, _ := io.ReadAll(loadResp.Body)
	log.Printf("Load response: %s", loadOutput)

	sourceRef := extractImageID(string(loadOutput))
	if sourceRef == "" {
		return fmt.Errorf("could not determine loaded image ID")
	}

	targetRef := img.FullRef(reg.Addr)
	log.Printf("Tagging image %s as %s", sourceRef, targetRef)
	if err := docker.ImageTag(ctx, sourceRef, targetRef); err != nil {
		return fmt.Errorf("failed to tag image: %v", err)
	}

	var opts image.PushOptions
	if reg.HasCreds() {
		opts.RegistryAuth = reg.Encode()
	}

	log.Printf("Pushing %s to registry...", targetRef)
	pushResp, err := docker.ImagePush(ctx, targetRef, opts)
	if err != nil {
		return fmt.Errorf("failed to push image: %v", err)
	}
	defer pushResp.Close()

	pushOutput, _ := io.ReadAll(pushResp)
	log.Printf("Push response: %s", string(pushOutput))

	log.Printf("Pushed with Docker: %s", targetRef)
	return nil
}

func extractImageID(loadOutput string) string {
	lines := strings.Split(loadOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Loaded image:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				return strings.TrimSpace(strings.Join(parts[1:], ":"))
			}
		}
		if strings.Contains(line, "sha256:") {
			parts := strings.Split(line, "sha256:")
			if len(parts) >= 2 {
				id := strings.TrimSpace(parts[1])
				if len(id) >= 12 {
					return "sha256:" + id[:12]
				}
			}
		}
	}
	return ""
}

func main() {
	reg := NewRegistry()
	img := NewImage()
	ctx := context.Background()

	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	container := client.Container().
		From("python:3.11-slim").
		WithNewFile("/app/hello.py", dagger.ContainerWithNewFileOpts{
			Contents:    "#!/usr/bin/env python3\nprint('Hello World from Dagger!')",
			Permissions: 0755,
		}).
		WithWorkdir("/app").
		WithEntrypoint([]string{"python3", "hello.py"})

	if useDagger() {
		log.Println("Using Dagger for push...")
		if err := pushWithDagger(ctx, client, container, reg, img); err != nil {
			log.Fatalf("Dagger push failed: %v", err)
		}
	} else {
		log.Println("Using Docker for push...")
		if err := pushWithDocker(ctx, container, reg, img); err != nil {
			log.Fatalf("Docker push failed: %v", err)
		}
	}
}
