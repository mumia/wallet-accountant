package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// renderer renders what the local cluster applies with the flux and helm
// binaries.
type renderer struct {
	ctx                                   context.Context
	helm, flux, cache, mirror, localChart string
	lock                                  Lock
}

func (r renderer) run(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(r.ctx, name, args...) //nolint:gosec // flux, helm and docker, named by the Makefile
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// cluster renders clusters/local the way Flux applies it: the `cluster`
// and `observability` (the Grafana stack) Kustomizations, then every Flux
// Kustomization they create, with their
// patches, components and substitutions. The env repo is copied first, with
// what `make local-push` adds (deploy/local/stage.sh): the app (clusters/local/
// build adding ../apps), and placeholders for this Mac's CA and the API spec.
func (r renderer) cluster(env string) (Found, error) {
	tmp, err := os.MkdirTemp("", "imageslock-")
	if err != nil {
		return Found{}, fmt.Errorf("render: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	for _, d := range []string{"platform", "apps", "clusters"} {
		if err := copyDir(filepath.Join(env, d), filepath.Join(tmp, d)); err != nil {
			return Found{}, err
		}
	}
	files := map[string]string{
		"clusters/local/apps/wallet-accountant/ca.crt": "",
		"clusters/local/swagger/openapi.yaml":          "",
		"clusters/local/build/kustomization.yaml":      "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\nresources:\n  - ../apps\n",
	}
	for f, content := range files {
		p := filepath.Join(tmp, f)
		if _, err := os.Stat(p); content == "" && err == nil {
			continue
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			return Found{}, fmt.Errorf("render: %w", err)
		}
	}

	// Each entry point, and every Flux Kustomization they create, in turn:
	// the opt-in Grafana stack (`observability`) creates its own.
	var all []byte
	built := map[string]bool{}
	queue := []fluxKustomization{
		{name: "cluster", path: "./clusters/local", file: "./clusters/local/flux-system/cluster.yaml"},
		{name: "observability", path: "./clusters/local/observability", file: "./clusters/local/flux-system/observability.yaml"},
	}
	for n := 0; len(queue) > 0; n++ {
		k := queue[0]
		queue = queue[1:]
		if built[k.name] {
			continue
		}
		built[k.name] = true
		if k.file == "" {
			data, err := yaml.Marshal(k.doc)
			if err != nil {
				return Found{}, fmt.Errorf("render: %w", err)
			}
			k.file = filepath.Join(tmp, fmt.Sprintf("flux-kustomization-%d.yaml", n))
			if err := os.WriteFile(k.file, data, 0o600); err != nil {
				return Found{}, fmt.Errorf("render: %w", err)
			}
		}
		out, err := r.run(tmp, r.flux, "build", "kustomization", k.name, "--path", k.path,
			"--kustomization-file", k.file, "--dry-run")
		if err != nil {
			return Found{}, err
		}
		all = append(append(all, "\n---\n"...), out...)
		found, err := Extract(out)
		if err != nil {
			return Found{}, err
		}
		for _, doc := range found.Docs {
			name := str(doc, "metadata", "name")
			if !strings.HasPrefix(str(doc, "apiVersion"), "kustomize.toolkit.fluxcd.io/") || doc["kind"] != "Kustomization" || built[name] {
				continue
			}
			queue = append(queue, fluxKustomization{name: name, path: "./" + strings.TrimPrefix(str(doc, "spec", "path"), "./"), doc: doc})
		}
	}
	return Extract(all)
}

// fluxKustomization is one Flux Kustomization to render: an entry point's
// file, or a document another one created.
type fluxKustomization struct {
	name, path, file string
	doc              map[string]any
}

func (r renderer) isLocalChart(c Chart) bool {
	if r.localChart == "" {
		return false
	}
	data, err := readFile(filepath.Join(r.localChart, "Chart.yaml"))
	if err != nil {
		return false
	}
	var meta struct {
		Name string `yaml:"name"`
	}
	return yaml.Unmarshal(data, &meta) == nil && meta.Name == c.Name
}

// template renders a HelmRelease's chart with its values. Wallet
// Accountant's chart comes from this working tree; a locked chart is pulled
// from the source images.lock gives (or, failing that, from the cluster's
// registry) into the cache.
func (r renderer) template(rel Release) ([]byte, error) {
	chart := r.localChart
	if !r.isLocalChart(rel.Chart) {
		var err error
		if chart, err = r.pull(rel.Chart); err != nil {
			return nil, err
		}
	}
	values, err := yaml.Marshal(rel.Values)
	if err != nil {
		return nil, fmt.Errorf("values of %s: %w", rel.Name, err)
	}
	f, err := os.CreateTemp("", "imageslock-values-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("values of %s: %w", rel.Name, err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	_, err = f.Write(values)
	if err = errors.Join(err, f.Close()); err != nil {
		return nil, fmt.Errorf("values of %s: %w", rel.Name, err)
	}
	return r.run("", r.helm, "template", rel.Name, chart, "--namespace", rel.Namespace, "--values", f.Name())
}

func (r renderer) pull(c Chart) (string, error) {
	var locked LockedChart
	for _, l := range r.lock.Charts {
		if l.Name == c.Name && l.Version == c.Version {
			locked = l
		}
	}
	if locked.Name == "" {
		return "", fmt.Errorf("chart %s %s is not in images.lock, so its images were not checked", c.Name, c.Version)
	}
	dir := filepath.Join(r.cache, c.Name+"-"+c.Version)
	if matches, _ := filepath.Glob(filepath.Join(dir, "*.tgz")); len(matches) == 1 {
		return matches[0], nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("chart cache: %w", err)
	}
	var err error
	if strings.HasPrefix(locked.Source, "oci://") {
		_, err = r.run("", r.helm, "pull", locked.Source, "--version", c.Version, "--destination", dir)
	} else {
		_, err = r.run("", r.helm, "pull", c.Name, "--repo", locked.Source, "--version", c.Version, "--destination", dir)
	}
	if err != nil {
		if _, err2 := r.run("", r.helm, "pull", "oci://"+r.mirror+"/charts/"+c.Name, "--version", c.Version,
			"--plain-http", "--destination", dir); err2 != nil {
			return "", errors.Join(err, err2)
		}
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*.tgz"))
	if len(matches) != 1 {
		return "", fmt.Errorf("chart %s %s: no package in %s", c.Name, c.Version, dir)
	}
	return matches[0], nil
}

func copyDir(src, dst string) error {
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err //nolint:wrapcheck // wrapped once, below
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		data, err := readFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		return fmt.Errorf("copy %s: %w", src, err)
	}
	return nil
}
