package assets

import (
	"embed"
	"path"
)

//go:generate protoc -Ibundled/ --include_imports -ogen/google_api_annotations.fdp bundled/google/api/annotations.proto
//go:generate protoc -Ibundled/ --include_imports -ogen/google_protobuf_empty.fdp bundled/google/protobuf/empty.proto
//go:generate protoc -Ibundled/ --include_imports -ogen/google_protobuf_timestamp.fdp bundled/google/protobuf/timestamp.proto
//go:generate protoc -Ibundled/ --include_imports -ogen/google_protobuf_duration.fdp bundled/google/protobuf/duration.proto
//go:generate protoc -Ibundled/ --include_imports -ogen/protoc-gen-openapiv2_options_annotations.fdp bundled/protoc-gen-openapiv2/options/annotations.proto
//go:generate cp ../docgen/templates/api.md gen/api.md
// Assets contains gen project assets.
//
//go:embed gen/*
var Assets embed.FS

// ReadFile returns a file from the assets.
func ReadFile(name string) ([]byte, error) {
	return Assets.ReadFile(path.Join("gen", name))
}
