package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

// Found is what a set of rendered manifests uses.
type Found struct {
	// Images are the values of every `image:` (and CloudNativePG
	// `imageName:`) string, anywhere in any document, in document order.
	Images   []string
	Releases []Release
	Sources  []Source
	// Docs are the decoded documents, for the caller to look further.
	Docs []map[string]any
}

// Release is a Flux HelmRelease with the chart it installs and where Flux
// fetches that chart from.
type Release struct {
	Name, Namespace string
	Chart           Chart
	Source          string
	Values          map[string]any
}

// Source is a Flux source (HelmRepository, OCIRepository, ...) and its URL.
type Source struct {
	Kind, Namespace, Name, URL, Tag string
}

// Extract reads multi-document YAML.
func Extract(data []byte) (Found, error) {
	var found Found
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var node yaml.Node
		err := dec.Decode(&node)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Found{}, fmt.Errorf("rendered YAML: %w", err)
		}
		found.Images = append(found.Images, images(&node)...)
		var doc map[string]any
		if err := node.Decode(&doc); err != nil || doc == nil {
			continue
		}
		found.Docs = append(found.Docs, doc)
	}
	for _, doc := range found.Docs {
		kind, _ := doc["kind"].(string)
		apiVersion, _ := doc["apiVersion"].(string)
		if !strings.HasPrefix(apiVersion, "source.toolkit.fluxcd.io/") {
			continue
		}
		found.Sources = append(found.Sources, Source{
			Kind: kind, Namespace: str(doc, "metadata", "namespace"), Name: str(doc, "metadata", "name"),
			URL: str(doc, "spec", "url"), Tag: str(doc, "spec", "ref", "tag"),
		})
	}
	for _, doc := range found.Docs {
		if doc["kind"] != "HelmRelease" || !strings.HasPrefix(str(doc, "apiVersion"), "helm.toolkit.fluxcd.io/") {
			continue
		}
		r := Release{Name: str(doc, "metadata", "name"), Namespace: str(doc, "metadata", "namespace")}
		if ns := str(doc, "spec", "targetNamespace"); ns != "" {
			r.Namespace = ns
		}
		r.Values, _ = get(doc, "spec", "values").(map[string]any)
		if name := str(doc, "spec", "chartRef", "name"); name != "" {
			ns := str(doc, "spec", "chartRef", "namespace")
			if ns == "" {
				ns = r.Namespace
			}
			src := found.source(str(doc, "spec", "chartRef", "kind"), ns, name)
			r.Source = src.URL
			r.Chart = Chart{Name: path.Base(src.URL), Version: src.Tag}
		} else {
			ns := str(doc, "spec", "chart", "spec", "sourceRef", "namespace")
			if ns == "" {
				ns = r.Namespace
			}
			src := found.source(str(doc, "spec", "chart", "spec", "sourceRef", "kind"), ns, str(doc, "spec", "chart", "spec", "sourceRef", "name"))
			r.Source = src.URL
			r.Chart = Chart{Name: str(doc, "spec", "chart", "spec", "chart"), Version: str(doc, "spec", "chart", "spec", "version")}
		}
		found.Releases = append(found.Releases, r)
	}
	return found, nil
}

// UpstreamSources lists every Flux source that does not fetch from the local
// registry: offline, it could not be fetched.
func (f Found) UpstreamSources(localRegistry string) []string {
	var problems []string
	for _, s := range f.Sources {
		if !strings.HasPrefix(s.URL, "oci://"+localRegistry+"/") && s.URL != "oci://"+localRegistry {
			problems = append(problems, fmt.Sprintf("%s %s/%s fetches %s, not from %s", s.Kind, s.Namespace, s.Name, s.URL, localRegistry))
		}
	}
	return problems
}

func (f Found) source(kind, namespace, name string) Source {
	for _, s := range f.Sources {
		if s.Kind == kind && s.Namespace == namespace && s.Name == name {
			return s
		}
	}
	return Source{}
}

// images walks a node in document order for image strings.
func images(n *yaml.Node) []string {
	var out []string
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			out = append(out, images(c)...)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if (k.Value == "image" || k.Value == "imageName") && v.Kind == yaml.ScalarNode && v.Value != "" {
				out = append(out, v.Value)
				continue
			}
			out = append(out, images(v)...)
		}
	}
	return out
}

func get(doc map[string]any, keys ...string) any {
	var v any = doc
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

func str(doc map[string]any, keys ...string) string {
	s, _ := get(doc, keys...).(string)
	return s
}
