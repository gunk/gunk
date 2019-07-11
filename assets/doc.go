package assets

//go:generate protoc -Ibundled/ --include_imports -ogenerated/google_api_annotations.fdp bundled/google/api/annotations.proto
//go:generate protoc -Ibundled/ --include_imports -ogenerated/google_protobuf_empty.fdp bundled/google/protobuf/empty.proto
//go:generate protoc -Ibundled/ --include_imports -ogenerated/google_protobuf_timestamp.fdp bundled/google/protobuf/timestamp.proto
//go:generate protoc -Ibundled/ --include_imports -ogenerated/google_protobuf_duration.fdp bundled/google/protobuf/duration.proto
//go:generate protoc -Ibundled/ --include_imports -ogenerated/protoc-gen-swagger_options_annotations.fdp bundled/protoc-gen-swagger/options/annotations.proto
//go:generate cp ../docgen/templates/api.md generated/api.md
//go:generate cp ../docgen/templates/annex.md generated/annex.md
//go:generate vfsgendev -source="github.com/gunk/gunk/assets".Assets
