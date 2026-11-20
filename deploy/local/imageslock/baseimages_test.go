package main

import (
	"slices"
	"testing"
)

func TestDockerfileImagesListsBaseImagesNotStagesOrScratch(t *testing.T) {
	got := DockerfileImages([]byte(`# a comment
FROM scratch AS vendor
FROM --platform=$BUILDPLATFORM golang:1.27.4-alpine AS build
RUN echo FROM nothing
from build AS test
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out /out
`))
	want := []string{"golang:1.27.4-alpine", "gcr.io/distroless/static-debian13:nonroot"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
