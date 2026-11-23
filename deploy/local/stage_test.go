package localtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// stageBuild is what `make local-build` (the app, and the stand-in UI when
// asked) recorded; uiBuild is what wallet-accountant-ui's `make local-deploy`
// recorded ("" for no file).
type stageInput struct {
	build    string
	uiBuild  string
	inRegist []string // "<repository>:<tag>" the registry stand-in has
}

// stage runs deploy/local/stage.sh with mkcert and curl stand-ins (curl
// answers from in.inRegist) and returns the build kustomization it wrote
// ("" if none) and its output.
func stage(t *testing.T, in stageInput) (kustomization, out string) {
	t.Helper()
	bin := t.TempDir()
	ca := t.TempDir()
	if err := os.WriteFile(filepath.Join(ca, "rootCA.pem"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	stubs := map[string]string{
		"mkcert": "#!/bin/sh\necho \"" + ca + "\"\n",
		// The last argument is .../v2/<repository>/manifests/<tag>.
		"curl": "#!/bin/sh\nfor a; do url=$a; done\n" +
			"ref=$(echo \"$url\" | sed -n 's#.*/v2/\\(.*\\)/manifests/\\(.*\\)#\\1:\\2#p')\n" +
			"case \" $IN_REGISTRY \" in *\" $ref \"*) exit 0 ;; esac\nexit 22\n",
	}
	for name, body := range stubs {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil { //nolint:gosec // G306: an executable stand-in in the test's own directory
			t.Fatal(err)
		}
	}
	env := t.TempDir()
	for _, d := range []string{"clusters/local/apps/wallet-accountant", "clusters/local/swagger", "clusters/local/build"} {
		if err := os.MkdirAll(filepath.Join(env, d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	build := filepath.Join(dir, "local-build.env")
	if err := os.WriteFile(build, []byte(in.build), 0o600); err != nil {
		t.Fatal(err)
	}
	uiBuild := filepath.Join(dir, "ui-local-build.env")
	if in.uiBuild != "" {
		if err := os.WriteFile(uiBuild, []byte(in.uiBuild), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	staged := filepath.Join(dir, "stage")
	cmd := exec.CommandContext(t.Context(), "deploy/local/stage.sh", staged) //nolint:gosec // G204: the test's own arguments
	cmd.Dir = "../.."
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ENV_REPO="+env, "LOCAL_BUILD="+build, "LOCAL_UI_BUILD="+uiBuild,
		"LOCAL_REGISTRY=wa-registry", "LOCAL_REGISTRY_PORT=5050",
		"IN_REGISTRY="+strings.Join(in.inRegist, " "),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("stage.sh: %v\n%s", err, output)
	}
	written, err := os.ReadFile(filepath.Join(staged, "clusters/local/build/kustomization.yaml")) //nolint:gosec // G304: the test's own file
	if err == nil {
		kustomization = string(written)
	}
	return kustomization, string(output)
}

const appBuild = "WALLET_ACCOUNTANT_VERSION=0.3.0-local.20261123100000\nWALLET_ACCOUNTANT_IMAGE_TAG=local-20261123100000\n"

const app = "wallet-accountant:local-20261123100000"

// uiImage is the UI slot's image in a staged kustomization ("" when the slot
// stays off).
func uiImage(kustomization string) string {
	var repo, tag string
	for line := range strings.SplitSeq(kustomization, "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "repository: "); ok {
			repo = v
		}
		if v, ok := strings.CutPrefix(line, "tag: "); ok {
			tag = v
		}
	}
	if repo == "" {
		return ""
	}
	return repo + ":" + tag
}

func TestStageTurnsTheUISlotOnWithTheNewestUIBuild(t *testing.T) {
	standIn := "WALLET_ACCOUNTANT_UI_IMAGE=wallet-accountant-stand-in-ui\nWALLET_ACCOUNTANT_UI_TAG=local-20261123100000\n"
	for _, tc := range []struct {
		name string
		in   stageInput
		want string
	}{
		{
			name: "no UI build",
			in:   stageInput{build: appBuild, inRegist: []string{app}},
			want: "",
		},
		{
			name: "the stand-in UI only",
			in:   stageInput{build: appBuild + standIn, inRegist: []string{app, "wallet-accountant-stand-in-ui:local-20261123100000"}},
			want: "wa-registry:5000/wallet-accountant-stand-in-ui:local-20261123100000",
		},
		{
			name: "the UI repo's build only",
			in: stageInput{
				build:    appBuild,
				uiBuild:  "WALLET_ACCOUNTANT_UI_IMAGE=wallet-accountant-ui\nWALLET_ACCOUNTANT_UI_TAG=local-20261123110000\n",
				inRegist: []string{app, "wallet-accountant-ui:local-20261123110000"},
			},
			want: "wa-registry:5000/wallet-accountant-ui:local-20261123110000",
		},
		{
			name: "the UI repo's build, newer than the stand-in",
			in: stageInput{
				build:    appBuild + standIn,
				uiBuild:  "WALLET_ACCOUNTANT_UI_IMAGE=wallet-accountant-ui\nWALLET_ACCOUNTANT_UI_TAG=local-20261123110000\n",
				inRegist: []string{app, "wallet-accountant-stand-in-ui:local-20261123100000", "wallet-accountant-ui:local-20261123110000"},
			},
			want: "wa-registry:5000/wallet-accountant-ui:local-20261123110000",
		},
		{
			name: "the stand-in, newer than the UI repo's build",
			in: stageInput{
				build:    appBuild + standIn,
				uiBuild:  "WALLET_ACCOUNTANT_UI_IMAGE=wallet-accountant-ui\nWALLET_ACCOUNTANT_UI_TAG=local-20261123090000\n",
				inRegist: []string{app, "wallet-accountant-stand-in-ui:local-20261123100000", "wallet-accountant-ui:local-20261123090000"},
			},
			want: "wa-registry:5000/wallet-accountant-stand-in-ui:local-20261123100000",
		},
		{
			name: "the newer UI build is no longer in the registry",
			in: stageInput{
				build:    appBuild + standIn,
				uiBuild:  "WALLET_ACCOUNTANT_UI_IMAGE=wallet-accountant-ui\nWALLET_ACCOUNTANT_UI_TAG=local-20261123110000\n",
				inRegist: []string{app, "wallet-accountant-stand-in-ui:local-20261123100000"},
			},
			want: "wa-registry:5000/wallet-accountant-stand-in-ui:local-20261123100000",
		},
		{
			name: "no UI build is in the registry",
			in: stageInput{
				build:    appBuild,
				uiBuild:  "WALLET_ACCOUNTANT_UI_IMAGE=wallet-accountant-ui\nWALLET_ACCOUNTANT_UI_TAG=local-20261123110000\n",
				inRegist: []string{app},
			},
			want: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kustomization, out := stage(t, tc.in)
			if !strings.Contains(kustomization, "WALLET_ACCOUNTANT_IMAGE_TAG=local-20261123100000") {
				t.Fatalf("no app staged:\n%s\n%s", kustomization, out)
			}
			if got := uiImage(kustomization); got != tc.want {
				t.Errorf("UI slot = %q, want %q\n%s\n%s", got, tc.want, kustomization, out)
			}
		})
	}
}

// Without the app's build, nothing is deployed: not the UI either, since
// the UI slot belongs to the app's release.
func TestStageDeploysNoUIWithoutTheApp(t *testing.T) {
	kustomization, out := stage(t, stageInput{
		build:    appBuild,
		uiBuild:  "WALLET_ACCOUNTANT_UI_IMAGE=wallet-accountant-ui\nWALLET_ACCOUNTANT_UI_TAG=local-20261123110000\n",
		inRegist: []string{"wallet-accountant-ui:local-20261123110000"},
	})
	if kustomization != "" {
		t.Errorf("staged a build without the app's image:\n%s\n%s", kustomization, out)
	}
}
