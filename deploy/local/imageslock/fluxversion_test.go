package main

import (
	"strings"
	"testing"
)

const variablesFixture = `
variable "k3s_version" {
  description = "k3s release."
  type        = string
  # renovate: datasource=github-releases depName=k3s-io/k3s
  default = "v1.35.5+k3s1"
}

variable "flux_version" {
  description = "Flux release bootstrapped into the cluster, pinned (keep in step with images.lock)."
  type        = string
  # renovate: datasource=github-releases depName=fluxcd/flux2
  default = "v2.10.0"
}

variable "git_branch" {
  default = "main"
}
`

func TestFluxVersionIsInfraHomelabsFluxVersion(t *testing.T) {
	got, err := FluxVersion([]byte(variablesFixture))
	if err != nil {
		t.Fatal(err)
	}
	if got != "v2.10.0" {
		t.Errorf("FluxVersion = %q, want v2.10.0", got)
	}
}

func TestFluxVersionFailsWithoutThePin(t *testing.T) {
	if _, err := FluxVersion([]byte(`variable "flux_version" {
  type = string
}
`)); err == nil {
		t.Error("FluxVersion without a default: no error")
	}
	if got, err := FluxVersion([]byte(`variable "flux_version" {
  type = string
}

variable "git_branch" {
  default = "main"
}
`)); err == nil {
		t.Errorf("FluxVersion took the next variable's default %q", got)
	}
	if _, err := FluxVersion([]byte(`variable "k3s_version" {
  default = "v1.35.5+k3s1"
}
`)); err == nil {
		t.Error("FluxVersion without flux_version: no error")
	}
}

func TestFluxCLIMustBeThePinnedFlux(t *testing.T) {
	if err := FluxCLIMatches("v2.10.0", []byte("flux: v2.10.0\n")); err != nil {
		t.Errorf("the pinned CLI: %v", err)
	}
	err := FluxCLIMatches("v2.10.1", []byte("flux: v2.10.0\n"))
	if err == nil || !strings.Contains(err.Error(), "v2.10.1") {
		t.Errorf("another CLI: %v, want an error naming v2.10.1", err)
	}
	if err := FluxCLIMatches("v2.10.0", []byte("something else\n")); err == nil {
		t.Error("unreadable `flux version --client`: no error")
	}
}
