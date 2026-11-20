package main

import (
	"bufio"
	"bytes"
	"strings"
)

// DockerfileImages lists a Dockerfile's base images: every FROM that is not
// scratch or an earlier stage.
func DockerfileImages(dockerfile []byte) []string {
	var out []string
	stages := map[string]bool{"scratch": true}
	sc := bufio.NewScanner(bytes.NewReader(dockerfile))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 || !strings.EqualFold(fields[0], "FROM") {
			continue
		}
		fields = fields[1:]
		for len(fields) > 0 && strings.HasPrefix(fields[0], "--") {
			fields = fields[1:]
		}
		if len(fields) == 0 {
			continue
		}
		image := fields[0]
		if !stages[strings.ToLower(image)] {
			out = append(out, image)
		}
		if len(fields) >= 3 && strings.EqualFold(fields[1], "AS") {
			stages[strings.ToLower(fields[2])] = true
		}
	}
	return out
}
