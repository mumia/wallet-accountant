package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// FluxVersionFile is where the env repo pins its Flux release: the one
// production bootstraps (infra/homelab's flux_version), and so the flux CLI
// every check and the local cluster use, and the version images.lock's Flux
// controllers come from. This repo pins no Flux version of its own.
const FluxVersionFile = "infra/homelab/variables.tf"

var (
	fluxVersionBlock = regexp.MustCompile(`(?ms)^variable\s+"flux_version"\s*\{\n.*?^\}`)
	defaultValue     = regexp.MustCompile(`(?m)^\s*default\s*=\s*"([^"]+)"`)
)

// FluxVersion reads flux_version's default from infra/homelab/variables.tf.
func FluxVersion(variablesTF []byte) (string, error) {
	block := fluxVersionBlock.Find(variablesTF)
	m := defaultValue.FindSubmatch(block)
	if m == nil {
		return "", errors.New(FluxVersionFile + ": no flux_version variable with a default")
	}
	return string(m[1]), nil
}

var fluxClientVersion = regexp.MustCompile(`(?m)^flux:\s*(v?\S+)\s*$`)

// FluxCLIMatches fails unless `flux version --client` printed the pinned
// version: `flux install --export` renders the controllers of the CLI's own
// release, which images.lock must match.
func FluxCLIMatches(pinned string, versionOutput []byte) error {
	m := fluxClientVersion.FindSubmatch(versionOutput)
	if m == nil {
		return fmt.Errorf("flux version --client: no version in %q", versionOutput)
	}
	if got := "v" + strings.TrimPrefix(string(m[1]), "v"); got != pinned {
		return fmt.Errorf("the flux CLI is %s, but the env repo pins Flux %s (%s, flux_version): install flux %s", got, pinned, FluxVersionFile, pinned)
	}
	return nil
}
