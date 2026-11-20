package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// MirrorProblems checks the k3d config (clusters/local/k3d.yaml): k3s's
// registries.yaml must send every registry images.lock mirrors to the local
// registry, keeping the upstream name as a path prefix (the layout
// `make local-prefetch` writes: the rewrite "^" -> "<registry>/" prefixes it,
// with no "$", which k3d would expand as an environment variable), and must fail closed: no fallback to the
// upstream registry, and a "*" mirror for every other registry.
func MirrorProblems(k3dConfig []byte, lock Lock, localRegistry string) []string {
	var cfg struct {
		Options struct {
			K3s struct {
				ExtraArgs []struct {
					Arg string `yaml:"arg"`
				} `yaml:"extraArgs"`
			} `yaml:"k3s"`
		} `yaml:"options"`
		Registries struct {
			Config string `yaml:"config"`
		} `yaml:"registries"`
	}
	if err := yaml.Unmarshal(k3dConfig, &cfg); err != nil {
		return []string{fmt.Sprintf("k3d config: %v", err)}
	}
	var registries struct {
		Mirrors map[string]struct {
			Endpoint []string          `yaml:"endpoint"`
			Rewrite  map[string]string `yaml:"rewrite"`
		} `yaml:"mirrors"`
	}
	if err := yaml.Unmarshal([]byte(cfg.Registries.Config), &registries); err != nil {
		return []string{fmt.Sprintf("registries.yaml (k3d registries.config): %v", err)}
	}
	endpoint := "http://" + localRegistry

	var problems []string
	failClosed := false
	for _, a := range cfg.Options.K3s.ExtraArgs {
		if a.Arg == "--disable-default-registry-endpoint" {
			failClosed = true
		}
	}
	if !failClosed {
		problems = append(problems, "k3s: --disable-default-registry-endpoint is not set, so containerd falls back to upstream registries")
	}
	if m, ok := registries.Mirrors["*"]; !ok || !slices.Equal(m.Endpoint, []string{endpoint}) {
		problems = append(problems, fmt.Sprintf(`registries.yaml: no "*" mirror to %s, so other registries are pulled from upstream`, endpoint))
	}

	upstream := map[string]bool{}
	for _, ref := range lock.Images {
		upstream[ref.Registry()] = true
	}
	names := make([]string, 0, len(upstream))
	for r := range upstream {
		names = append(names, r)
	}
	sort.Strings(names)
	for _, r := range names {
		m, ok := registries.Mirrors[r]
		for from, to := range m.Rewrite {
			if strings.Contains(from+to, "$") {
				problems = append(problems, fmt.Sprintf(`registries.yaml: a "$" in k3d's config is an environment variable to k3d, which replaces it (with nothing): %s's rewrite %q: %q`, r, from, to))
			}
		}
		if !ok || !slices.Equal(m.Endpoint, []string{endpoint}) || len(m.Rewrite) != 1 || m.Rewrite["^"] != r+"/" {
			problems = append(problems, fmt.Sprintf(`registries.yaml: %s is not mirrored to %s with rewrite "^": "%s/"`, r, endpoint, r))
		}
	}
	return problems
}
