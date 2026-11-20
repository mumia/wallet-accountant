// Command imageslock keeps the env repo's images.lock honest (ADR-0019): the
// local cluster runs offline only if everything it pulls was mirrored, and
// `make local-prefetch` mirrors exactly what images.lock lists.
//
//	imageslock check  -env <env repo> -helm <helm> [-chart <dir>] [-dockerfile <file>]...
//	    Renders what clusters/local applies (every Flux Kustomization, with
//	    `flux build --dry-run`, and every HelmRelease's chart, with `helm
//	    template`), plus `flux install`, the k3d config and the Dockerfiles,
//	    and fails when an image or chart is missing from images.lock, a Flux
//	    source fetches from outside the cluster's registry, or k3s's
//	    registries.yaml lets a pull reach an upstream registry.
//	imageslock covers -lock <images.lock>
//	    Reads image references on stdin (a running node's images, say) and
//	    fails on any images.lock lacks.
//	imageslock list -lock <images.lock>
//	    Prints `image <ref>`, `host-image <ref>` and `chart <name> <source>
//	    <version>` lines, for `make local-prefetch`.
//	imageslock pin -lock <images.lock>
//	    Adds the digest to every image line that has none (needs the network).
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// localRegistry is the k3d registry as the cluster sees it (clusters/local/k3d.yaml).
const localRegistry = "wa-registry:5000"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: imageslock check|covers|list|pin [flags]")
		os.Exit(2)
	}
	ctx := context.Background()
	var err error
	switch cmd, args := os.Args[1], os.Args[2:]; cmd {
	case "check":
		err = runCheck(ctx, args)
	case "covers":
		err = runCovers(args)
	case "list":
		err = runList(args)
	case "pin":
		err = runPin(ctx, args)
	default:
		err = fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "imageslock:", err)
		os.Exit(1)
	}
}

type fileList []string

func (d *fileList) String() string { return strings.Join(*d, ",") }

func (d *fileList) Set(s string) error {
	*d = append(*d, s)
	return nil
}

func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the paths are this tool's arguments, given by the Makefile
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	return data, nil
}

func runCheck(ctx context.Context, args []string) error {
	fl := flag.NewFlagSet("check", flag.ExitOnError)
	env := fl.String("env", "", "the env repo checkout (wallet-accountant-deploy)")
	helm := fl.String("helm", "helm", "the helm binary")
	flux := fl.String("flux", "flux", "the flux binary (its version is the Flux that `flux install` installs)")
	chartDir := fl.String("chart", "", "Wallet Accountant's chart, rendered from this working tree")
	cache := fl.String("cache", "bin/charts-cache", "where pulled charts are kept")
	mirror := fl.String("mirror", "localhost:5050", "the cluster's registry from this machine, tried when a chart's source is unreachable")
	verbose := fl.Bool("v", false, "list every image and chart found")
	var files fileList
	fl.Var(&files, "dockerfile", "a Dockerfile whose base images must be locked (repeatable)")
	_ = fl.Parse(args)
	if *env == "" {
		return errors.New("check: -env is required")
	}
	lockData, err := readFile(filepath.Join(*env, "images.lock"))
	if err != nil {
		return err
	}
	lock, err := ParseLock(lockData)
	if err != nil {
		return err
	}
	k3dConfig, err := readFile(filepath.Join(*env, "clusters/local/k3d.yaml"))
	if err != nil {
		return err
	}
	r := renderer{ctx: ctx, helm: *helm, flux: *flux, cache: *cache, mirror: *mirror, lock: lock}
	if *chartDir != "" {
		if r.localChart, err = filepath.Abs(*chartDir); err != nil {
			return fmt.Errorf("chart: %w", err)
		}
	}

	rendered, err := r.cluster(*env)
	if err != nil {
		return err
	}
	problems := rendered.UpstreamSources(localRegistry)
	images := append([]string(nil), rendered.Images...)
	var charts []Chart
	for _, rel := range rendered.Releases {
		if !r.isLocalChart(rel.Chart) {
			charts = append(charts, rel.Chart)
		}
		out, err := r.template(rel)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		found, err := Extract(out)
		if err != nil {
			return fmt.Errorf("chart %s: %w", rel.Chart.Name, err)
		}
		images = append(images, found.Images...)
	}
	fluxInstall, err := r.run("", r.flux, "install", "--export")
	if err != nil {
		return err
	}
	for _, data := range [][]byte{fluxInstall, k3dConfig} {
		found, err := Extract(data)
		if err != nil {
			return err
		}
		images = append(images, found.Images...)
	}
	for _, f := range files {
		data, err := readFile(f)
		if err != nil {
			return err
		}
		images = append(images, DockerfileImages(data)...)
	}

	if *verbose {
		for _, i := range images {
			fmt.Println("image", i)
		}
		for _, c := range charts {
			fmt.Println("chart", c.Name, c.Version)
		}
	}
	problems = append(problems, Audit(lock, Rendered{Images: images, Charts: charts, LocalRegistry: localRegistry})...)
	problems = append(problems, MirrorProblems(k3dConfig, lock, localRegistry)...)
	for _, p := range problems {
		fmt.Println("FAIL:", p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d problem(s); the env repo's README (\"Offline\") says how to add an entry", len(problems))
	}
	fmt.Printf("imageslock: %d image references and %d charts, all in images.lock and mirrored\n", len(images), len(charts))
	return nil
}

func readLock(args []string, name string) (Lock, string, error) {
	fl := flag.NewFlagSet(name, flag.ExitOnError)
	path := fl.String("lock", "images.lock", "images.lock")
	_ = fl.Parse(args)
	data, err := readFile(*path)
	if err != nil {
		return Lock{}, "", err
	}
	lock, err := ParseLock(data)
	return lock, *path, err
}

func runCovers(args []string) error {
	lock, _, err := readLock(args, "covers")
	if err != nil {
		return err
	}
	var refs []string
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		if s := strings.TrimSpace(sc.Text()); s != "" {
			refs = append(refs, s)
		}
	}
	problems := Audit(lock, Rendered{Images: refs, LocalRegistry: localRegistry})
	for _, p := range problems {
		fmt.Println("FAIL:", p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d image(s) not in images.lock", len(problems))
	}
	return nil
}

func runList(args []string) error {
	lock, _, err := readLock(args, "list")
	if err != nil {
		return err
	}
	for _, r := range lock.Images {
		fmt.Println("image", r)
	}
	for _, r := range lock.HostImages {
		fmt.Println("host-image", r)
	}
	for _, c := range lock.Charts {
		fmt.Println("chart", c.Name, c.Source, c.Version)
	}
	return nil
}

var unpinned = regexp.MustCompile(`^(\s*-\s+)([a-z0-9][a-z0-9._/:-]*[A-Za-z0-9_.+-])(\s*(#.*)?)$`)

// runPin adds the digest of the image index upstream serves to every image
// line under images: or host-images: that has none.
func runPin(ctx context.Context, args []string) error {
	fl := flag.NewFlagSet("pin", flag.ExitOnError)
	path := fl.String("lock", "images.lock", "images.lock")
	_ = fl.Parse(args)
	data, err := readFile(*path)
	if err != nil {
		return err
	}
	lines := strings.SplitAfter(string(data), "\n")
	section := ""
	r := renderer{ctx: ctx}
	for i, line := range lines {
		body := strings.TrimRight(line, "\n")
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "#") && strings.Contains(line, ":") {
			section = strings.TrimSpace(strings.SplitN(line, ":", 2)[0])
		}
		m := unpinned.FindStringSubmatch(body)
		if (section != "images" && section != "host-images") || m == nil || strings.Contains(m[2], "@") {
			continue
		}
		raw, err := r.run("", "docker", "buildx", "imagetools", "inspect", "--raw", m[2])
		if err != nil {
			return err
		}
		sum := sha256.Sum256(raw)
		lines[i] = m[1] + m[2] + "@sha256:" + hex.EncodeToString(sum[:]) + m[3] + line[len(body):]
		fmt.Println("pinned", strings.TrimSpace(lines[i]))
	}
	if err := os.WriteFile(*path, []byte(strings.Join(lines, "")), 0o600); err != nil {
		return fmt.Errorf("pin: %w", err)
	}
	return nil
}
