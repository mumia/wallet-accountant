package main

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Lock is the env repo's images.lock: every Helm chart (by version) and every
// image (by tag and digest) the local cluster uses (ADR-0019). Images are
// mirrored into the cluster's registry; host images are pulled into the Mac's
// Docker, which runs or builds from them (k3d, the Dockerfiles).
// Production-only charts and images (prod-charts:, prod-images:) are there
// for Renovate, which bumps them with their manifests; nothing local uses or
// mirrors them.
type Lock struct {
	Charts     []LockedChart `yaml:"charts"`
	Images     []Ref         `yaml:"-"`
	HostImages []Ref         `yaml:"-"`
	ProdCharts []LockedChart `yaml:"-"`
	ProdImages []Ref         `yaml:"-"`
}

// LockedChart is one chart entry: its name as the HelmRelease gives it, the
// Helm repository URL or oci:// path it comes from, and its version.
type LockedChart struct {
	Name    string `yaml:"name"`
	Source  string `yaml:"source"`
	Version string `yaml:"version"`
}

// Chart is a chart a rendered manifest uses.
type Chart struct {
	Name    string
	Version string
}

// ParseLock reads images.lock. Every image must carry its digest.
func ParseLock(data []byte) (Lock, error) {
	var raw struct {
		Charts     []LockedChart `yaml:"charts"`
		Images     []string      `yaml:"images"`
		HostImages []string      `yaml:"host-images"`
		ProdCharts []LockedChart `yaml:"prod-charts"`
		ProdImages []string      `yaml:"prod-images"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Lock{}, fmt.Errorf("images.lock: %w", err)
	}
	lock := Lock{Charts: raw.Charts, ProdCharts: raw.ProdCharts}
	var errs []error
	for _, c := range append(slices.Clone(raw.Charts), raw.ProdCharts...) {
		if c.Name == "" || c.Source == "" || c.Version == "" {
			errs = append(errs, fmt.Errorf("images.lock: chart %q needs name, source and version", c.Name))
		}
	}
	parse := func(list []string) []Ref {
		var refs []Ref
		for _, s := range list {
			ref, err := ParseRef(s)
			switch {
			case err != nil:
				errs = append(errs, fmt.Errorf("images.lock: %w", err))
			case ref.Digest == "" || ref.Tag == "":
				errs = append(errs, fmt.Errorf("images.lock: %s needs a tag and a digest (<name>:<tag>@sha256:<digest>; `make images-lock-pin` adds it)", s))
			default:
				refs = append(refs, ref)
			}
		}
		return refs
	}
	lock.Images = parse(raw.Images)
	lock.HostImages = parse(raw.HostImages)
	lock.ProdImages = parse(raw.ProdImages)
	return lock, errors.Join(errs...)
}

// Ref is an image reference, normalized: Name always has its registry
// (docker.io/library/nginx, not nginx).
type Ref struct {
	Name   string
	Tag    string
	Digest string
}

var digestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

// ParseRef parses [registry/]repository[:tag][@digest].
func ParseRef(s string) (Ref, error) {
	var ref Ref
	rest := strings.TrimSpace(s)
	if name, digest, ok := strings.Cut(rest, "@"); ok {
		if !digestPattern.MatchString(digest) {
			return Ref{}, fmt.Errorf("image %q: bad digest", s)
		}
		ref.Digest, rest = digest, name
	}
	if i := strings.LastIndex(rest, ":"); i > strings.LastIndex(rest, "/") {
		ref.Tag, rest = rest[i+1:], rest[:i]
	}
	if rest == "" || strings.ContainsAny(rest, " \t$") {
		return Ref{}, fmt.Errorf("image %q: bad name", s)
	}
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) == 1 || (!strings.ContainsAny(parts[0], ".:") && parts[0] != "localhost") {
		parts = []string{"docker.io", rest}
	}
	if parts[0] == "index.docker.io" {
		parts[0] = "docker.io"
	}
	if parts[0] == "docker.io" && !strings.Contains(parts[1], "/") {
		parts[1] = "library/" + parts[1]
	}
	ref.Name = parts[0] + "/" + parts[1]
	if ref.Tag == "" && ref.Digest == "" {
		ref.Tag = "latest"
	}
	return ref, nil
}

// Registry is the registry part of the name (docker.io, ghcr.io, ...).
func (r Ref) Registry() string {
	registry, _, _ := strings.Cut(r.Name, "/")
	return registry
}

// Repository is the name without its registry.
func (r Ref) Repository() string {
	_, repo, _ := strings.Cut(r.Name, "/")
	return repo
}

func (r Ref) String() string {
	s := r.Name
	if r.Tag != "" {
		s += ":" + r.Tag
	}
	if r.Digest != "" {
		s += "@" + r.Digest
	}
	return s
}

// covers reports whether the locked ref is the image ref asks for: the same
// name, and the same tag and digest where ref names them.
func (r Ref) covers(ref Ref) bool {
	return r.Name == ref.Name &&
		(ref.Tag == "" || ref.Tag == r.Tag) &&
		(ref.Digest == "" || ref.Digest == r.Digest)
}

// Covers reports whether the lock (images or host images) has ref.
func (l Lock) Covers(ref Ref) bool {
	for _, locked := range append(l.Images[:len(l.Images):len(l.Images)], l.HostImages...) {
		if locked.covers(ref) {
			return true
		}
	}
	return false
}
