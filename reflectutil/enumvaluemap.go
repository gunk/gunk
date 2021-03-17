package reflectutil

// taken from https://github.com/golang/protobuf/blob/master/proto/registry.go
// TODO remove this

import (
	"strings"
	"sync"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/runtime/protoimpl"
)

// enumName is the name of an enum. For historical reasons, the enum name is
// neither the full Go name nor the full protobuf name of the enum.
// The name is the dot-separated combination of just the proto package that the
// enum is declared within followed by the Go type name of the generated enum.
type enumName = string // e.g., "my.proto.package.GoMessage_GoEnum"

// enumsByName maps enum values by name to their numeric counterpart.
type enumsByName = map[string]int32

var (
	enumCache     sync.Map // map[enumName]enumsByName
	numFilesCache sync.Map // map[protoreflect.FullName]int
)

func enumValueMap(s enumName) enumsByName {
	if v, ok := enumCache.Load(s); ok {
		return v.(enumsByName)
	}

	// Check whether the cache is stale. If the number of files in the current
	// package differs, then it means that some enums may have been recently
	// registered upstream that we do not know about.
	var protoPkg protoreflect.FullName
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		protoPkg = protoreflect.FullName(s[:i])
	}
	v, _ := numFilesCache.Load(protoPkg)
	numFiles, _ := v.(int)
	if protoregistry.GlobalFiles.NumFilesByPackage(protoPkg) == numFiles {
		return nil // cache is up-to-date; was not found earlier
	}

	// Update the enum cache for all enums declared in the given proto package.
	numFiles = 0
	protoregistry.GlobalFiles.RangeFilesByPackage(protoPkg, func(fd protoreflect.FileDescriptor) bool {
		walkEnums(fd, func(ed protoreflect.EnumDescriptor) {
			name := protoimpl.X.LegacyEnumName(ed)
			if _, ok := enumCache.Load(name); !ok {
				m := make(enumsByName)
				evs := ed.Values()
				for i := evs.Len() - 1; i >= 0; i-- {
					ev := evs.Get(i)
					m[string(ev.Name())] = int32(ev.Number())
				}
				enumCache.LoadOrStore(name, m)
			}
		})
		numFiles++
		return true
	})
	numFilesCache.Store(protoPkg, numFiles)

	// Check cache again for enum map.
	if v, ok := enumCache.Load(s); ok {
		return v.(enumsByName)
	}
	return nil
}

// walkEnums recursively walks all enums declared in d.
func walkEnums(d interface {
	Enums() protoreflect.EnumDescriptors
	Messages() protoreflect.MessageDescriptors
}, f func(protoreflect.EnumDescriptor)) {
	eds := d.Enums()
	for i := eds.Len() - 1; i >= 0; i-- {
		f(eds.Get(i))
	}
	mds := d.Messages()
	for i := mds.Len() - 1; i >= 0; i-- {
		walkEnums(mds.Get(i), f)
	}
}
