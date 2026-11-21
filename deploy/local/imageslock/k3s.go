package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// k3sReleases is where k3s publishes each release's k3s-images.txt: the
// images that release runs itself (pause, CoreDNS, Traefik, local-path,
// metrics-server, klipper). No manifest names them, so they are checked
// against this list: the local cluster pulls only through its mirror, and a
// k3s update without them leaves every pod in ContainerCreating.
const k3sReleases = "https://github.com/k3s-io/k3s/releases/download"

// K3sRelease is the k3s release of the k3d node image among images
// (rancher/k3s:v1.37.2-k3s1 is release v1.37.2+k3s1).
func K3sRelease(images []string) (string, bool) {
	for _, s := range images {
		ref, err := ParseRef(s)
		if err != nil || ref.Name != "docker.io/rancher/k3s" || ref.Tag == "" {
			continue
		}
		i := strings.LastIndex(ref.Tag, "-k3s")
		if i < 0 {
			continue
		}
		return ref.Tag[:i] + "+" + ref.Tag[i+1:], true
	}
	return "", false
}

// K3sImages fetches the k3s release's k3s-images.txt from baseURL.
func K3sImages(ctx context.Context, baseURL, release string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	url := baseURL + "/" + release + "/k3s-images.txt"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("k3s images: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("k3s images: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("k3s images: GET %s: %s", url, resp.Status)
	}
	var images []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if s := strings.TrimSpace(sc.Text()); s != "" {
			images = append(images, s)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("k3s images: %w", err)
	}
	return images, nil
}
