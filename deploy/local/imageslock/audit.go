package main

import (
	"fmt"
	"strings"
)

// Rendered is what the local cluster uses, as found in its rendered
// manifests, the charts they install, Flux's own manifests and the
// Dockerfiles.
type Rendered struct {
	Images []string
	Charts []Chart
	// LocalRegistry is the cluster's own registry (wa-registry:5000). Images
	// on it are local builds, which images.lock does not pin.
	LocalRegistry string
}

// Audit lists every image and chart in rendered that images.lock lacks, once
// each, in the order found.
func Audit(lock Lock, rendered Rendered) []string {
	var problems []string
	seen := map[string]bool{}
	report := func(p string) {
		if !seen[p] {
			seen[p] = true
			problems = append(problems, p)
		}
	}
	for _, s := range rendered.Images {
		// An unsubstituted ${VAR} is a value Flux fills in at reconcile time:
		// the local build's tag.
		if strings.Contains(s, "${") ||
			(rendered.LocalRegistry != "" && strings.HasPrefix(s, rendered.LocalRegistry+"/")) {
			continue
		}
		ref, err := ParseRef(s)
		if err != nil {
			report(err.Error())
			continue
		}
		if !lock.Covers(ref) {
			report(fmt.Sprintf("image %s is not in images.lock", ref))
		}
	}
	for _, c := range rendered.Charts {
		if !lock.hasChart(c) {
			report(fmt.Sprintf("chart %s %s is not in images.lock", c.Name, c.Version))
		}
	}
	return problems
}

func (l Lock) hasChart(c Chart) bool {
	for _, locked := range l.Charts {
		if locked.Name == c.Name && locked.Version == c.Version {
			return true
		}
	}
	return false
}
