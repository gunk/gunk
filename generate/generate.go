package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	"github.com/xo/ecosystem/proto/xo"
	"google.golang.org/genproto/googleapis/api/annotations"

	"github.com/gunk/gunk/config"
	"github.com/gunk/gunk/generate/downloader"
	"github.com/gunk/gunk/loader"
	"github.com/gunk/gunk/log"
	"github.com/gunk/gunk/protoutil"
	"github.com/gunk/gunk/reflectutil"
	"github.com/karelbilek/dirchanges"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// Run generates the specified Gunk packages via protobuf generators, writing
// the output files in the same directories.
func Run(dir string, args ...string) error {
	g := NewGenerator(dir)
	// Check that protoc exists, if not download it.
	pkgs, err := g.Load(args...)
	if err != nil {
		return fmt.Errorf("error loading packages: %w", err)
	}
	if len(pkgs) == 0 {
		return fmt.Errorf("no Gunk packages to generate")
	}
	if loader.PrintErrors(pkgs) > 0 {
		return fmt.Errorf("encountered package loading errors")
	}
	// Record the loaded packages in gunkPkgs.
	g.recordPkgs(pkgs...)
	// Cache of a package directory to its gunkconfig.
	pkgConfigs := map[string]*config.Config{}
	// Translate the packages from Gunk to Proto.
	for _, pkg := range pkgs {
		cfg, err := config.Load(pkg.Dir)
		if err != nil {
			return fmt.Errorf("unable to load gunkconfig: %w", err)
		}
		pkgConfigs[pkg.Dir] = cfg
		if err := g.translatePkg(pkg.PkgPath); err != nil {
			return fmt.Errorf("unable to translate pkg: %w", err)
		}
	}
	// hack: take protoc config from the first package
	firstPkg := pkgs[0]
	cfg := pkgConfigs[firstPkg.Dir]
	protocPath, err := downloader.CheckOrDownloadProtoc(cfg.ProtocPath, cfg.ProtocVersion)
	if err != nil {
		return fmt.Errorf("unable to check or download protoc: %w", err)
	}
	g.protoLoader.ProtocPath = protocPath
	// Load any non-Gunk proto dependencies.
	if err := g.loadProtoDeps(); err != nil {
		return fmt.Errorf("unable to load protodeps: %w", err)
	}
	// Run the code generators.
	protocPaths := make(map[string]string, len(pkgs))
	pkgGens := make(map[string][]config.Generator, len(pkgs))
	pkgPaths := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		cfg := pkgConfigs[pkg.Dir]
		protocPath, err := downloader.CheckOrDownloadProtoc(cfg.ProtocPath, cfg.ProtocVersion)
		if err != nil {
			return fmt.Errorf("unable to check or download protoc: %w", err)
		}
		protocPaths[pkg.PkgPath] = protocPath
		pkgGens[pkg.PkgPath] = cfg.Generators
		pkgPaths = append(pkgPaths, pkg.PkgPath)
	}
	if err := g.GeneratePkgs(pkgPaths, pkgGens, protocPaths); err != nil {
		return err
	}
	return nil
}

// FileDescriptorSet will load a single Gunk package, and return the
// proto FileDescriptor set of the Gunk package.
//
// Currently, we only generate a FileDescriptorSet for one Gunk package.
func FileDescriptorSet(dir string, args ...string) (*descriptorpb.FileDescriptorSet, error) {
	// TODO: share code with Run; much of this function is identical.
	g := NewGenerator(dir)
	pkgs, err := g.Load(args...)
	if err != nil {
		return nil, err
	}
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("can only get FileDescriptorSet for a single Gunk package")
	}
	if loader.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("encountered package loading errors")
	}
	// Record the loaded packages in gunkPkgs.
	g.recordPkgs(pkgs...)
	// Translate the packages from Gunk to Proto.
	for _, pkg := range pkgs {
		if err := g.translatePkg(pkg.PkgPath); err != nil {
			return nil, err
		}
	}
	// Load any non-Gunk proto dependencies.
	if err := g.loadProtoDeps(); err != nil {
		return nil, err
	}
	// Generate the filedescriptorset for the Gunk package.
	req := g.newCodeGenRequest(pkgs[0].PkgPath)
	fds := &descriptorpb.FileDescriptorSet{File: req.ProtoFile}
	return fds, nil
}

// NewGenerator returns an initialized Generator with the provided dir.
func NewGenerator(dir string) *Generator {
	return &Generator{
		Loader: loader.Loader{
			Dir:   dir,
			Fset:  token.NewFileSet(),
			Types: true,
		},
		gunkPkgs:    make(map[string]*loader.GunkPackage),
		allProto:    make(map[string]*descriptorpb.FileDescriptorProto),
		protoLoader: &loader.ProtoLoader{},
	}
}

type Generator struct {
	loader.Loader
	curPkg *loader.GunkPackage               // current package being translated or generated
	curPos token.Pos                         // current position of the token being evaluated
	gfile  *ast.File                         // current Go file being translated
	pfile  *descriptorpb.FileDescriptorProto // current protobuf file being translated into

	usedImports map[string]bool // imports being used for the current package
	// Maps from package import path to package information.
	gunkPkgs map[string]*loader.GunkPackage
	// imported proto files will be loaded using protoLoader
	// holds the absolute path passed to -I flag from protoc
	protoLoader *loader.ProtoLoader
	// All protobuf that has been translated currently.
	allProto map[string]*descriptorpb.FileDescriptorProto
	// Next indexes to use for message, service and enum.
	messageIndex int32
	serviceIndex int32
	enumIndex    int32
}

// recordPkgs records all provided packages and their imports in the gunkPkgs
// field and resolve proto.Package tags.
func (g *Generator) recordPkgs(pkgs ...*loader.GunkPackage) {
	for _, pkg := range pkgs {
		// capture proto.Package annotation
		for _, f := range pkg.GunkSyntax {
			for _, tag := range pkg.GunkTags[f] {
				switch s := tag.Type.String(); s {
				case "github.com/gunk/opt/proto.Package":
					pkg.ProtoName = constant.StringVal(tag.Value)
				}
			}
		}
		g.gunkPkgs[pkg.PkgPath] = pkg
		for _, ipkg := range pkg.Imports {
			g.recordPkgs(ipkg)
		}
	}
}

// configWithBinary contains the configuration passed in and the binary to use
// for protoc.
type configWithBinary struct {
	config.Generator
	binary *string
}

// actualCommand returns the command to invoke for protoc operations.
func (c configWithBinary) actualCommand() string {
	if c.binary == nil {
		return c.Command
	}
	return *c.binary
}

// findPkg resolves package names for languages with different naming
// requirements and restrictions.
func (g *Generator) findPkg(path string) (pkg *loader.GunkPackage, ok bool) {
	if p, ok := g.gunkPkgs[path]; ok {
		return p, true
	}
	for k, p := range g.gunkPkgs {
		switch path {
		case strings.ReplaceAll(k, ".", "/"):
			// Python does not allow '.' in package names.
			return p, true
		}
	}
	return nil, false
}

// GeneratePkg runs the proto files resulting from translating gunk packages
// through a code generator, such as protoc-gen-go to generate Go packages.
//
// It is provided as a wrapper of GeneratePkgs.
func (g *Generator) GeneratePkg(path string, gens []config.Generator, protocPath string) error {
	paths := []string{path}
	generators := map[string][]config.Generator{
		path: gens,
	}
	protocPaths := map[string]string{
		path: protocPath,
	}
	return g.GeneratePkgs(paths, generators, protocPaths)
}

// GeneratePkgs runs the proto files resulting from translating gunk packages
// through a code generator, such as protoc-gen-go to generate Go packages.
//
// Generated files are written to the same directory, next to the source gunk
// files.
func (g *Generator) GeneratePkgs(paths []string, gens map[string][]config.Generator, protocPath map[string]string) error {
	run := func(req *pluginpb.CodeGeneratorRequest, generators []config.Generator, path string) error {
		for _, gen := range generators {
			req := g.pruneIgnored(req, gen)
			switch {
			case gen.IsProtoc():
				if gen.PluginVersion != "" {
					return fmt.Errorf("cannot use pinned version with protoc option")
				}
				if err := g.generateProtoc(req, gen, protocPath[path]); err != nil {
					return fmt.Errorf("unable to generate protoc: %w", err)
				}
			default:
				c := configWithBinary{Generator: gen}
				if gen.PluginVersion != "" {
					has := downloader.Has(gen.Code())
					if !has {
						return fmt.Errorf("plugin %s does not support pinned versions", gen.Code())
					}
					bin, err := downloader.Download(gen.Code(), gen.PluginVersion)
					if err != nil {
						return err
					}
					c.binary = &bin
				}
				if err := g.generatePlugin(req, c); err != nil {
					return fmt.Errorf("unable to generate plugin: %w", err)
				}
			}
		}
		return nil
	}
	type generatorWithFiles struct {
		generator config.Generator
		files     []string
	}
	singleFiles := []generatorWithFiles{}
	for _, generators := range gens {
	iterGen:
		for _, gen := range generators {
			if !gen.Single {
				continue
			}
			for _, v := range singleFiles {
				if reflect.DeepEqual(gen, v.generator) {
					// Don't redo identical generations.
					continue iterGen
				}
			}
			singleFile := generatorWithFiles{
				generator: gen,
				files:     []string{},
			}
			if len(singleFile.files) != 0 {
				continue
			}
			configAbs, err := filepath.Abs(gen.ConfigDir)
			if err != nil {
				return fmt.Errorf(
					"unable to get absolute directory of config path %q: %w",
					gen.ConfigDir, err,
				)
			}
			for _, path := range paths {
				pathAbs, err := filepath.Abs(g.gunkPkgs[path].Dir)
				if err != nil {
					return fmt.Errorf(
						"unable to get absolute directory of file path %q: %w",
						path, err,
					)
				}
				if strings.HasPrefix(pathAbs, configAbs) {
					singleFile.files = append(singleFile.files, path)
				}
			}
			singleFiles = append(singleFiles, singleFile)
		}
	}
	for _, v := range singleFiles {
		log.Verbosef("generator %s", v.generator.Command)
		req := g.newCodeGenRequest(v.files...)
		generators := []config.Generator{v.generator}
		if err := run(req, generators, ""); err != nil {
			return fmt.Errorf(
				"unable to generate_single on %s:\n\tFiles: %v\n\tError: %w",
				v.generator.Command, v.files, err,
			)
		}
	}
	for _, path := range paths {
		log.Verbosef("%s", path)
		// It is fine to pass the pluginpb.CodeGeneratorRequest to every protoc
		// generator unaltered; this is what protoc does when calling out to the
		// generators and the generators should already handle the case where they
		// have nothing to do.
		req := g.newCodeGenRequest(path)
		generators := make([]config.Generator, 0, len(gens[path]))
		for _, gen := range gens[path] {
			if gen.Single {
				continue
			}
			generators = append(generators, gen)
		}
		if err := run(req, generators, path); err != nil {
			return fmt.Errorf("unable to generate pkg %s: %w", path, err)
		}
	}
	return nil
}

// generateProtoc invokes protoc to generate the package specified in the
// CodeGeneratorRequest and applies post processing if applicable. It expects
// exactly one file to be requested in CodeGeneratorRequest.
func (g *Generator) generateProtoc(req *pluginpb.CodeGeneratorRequest, gen config.Generator, protocCommandPath string) error {
	// Default location to output protoc generated files.
	ftgs := req.GetFileToGenerate()
	switch len(ftgs) {
	case 0:
		return fmt.Errorf("unexpected length of ftgs, expected 1, got 0")
	case 1:
		// Proceed.
	default:
		return fmt.Errorf("protoc can only be invoked one file at a time")
	}
	// req.GetFileToGenerate() is always just 1 field, as we create the request
	// and it has just the one proto file
	ftg := ftgs[0]
	mainPkgPath, basename := filepath.Split(ftg)
	mainPkgPath = filepath.Clean(mainPkgPath)
	mainPkg, ok := g.gunkPkgs[mainPkgPath]
	if !ok {
		return fmt.Errorf("failed to get main package: %s", mainPkgPath)
	}
	// Make a copy of the slice, as we may modify the elements within in the
	// pf2 copying below.
	fds := &descriptorpb.FileDescriptorSet{}
	fds.File = make([]*descriptorpb.FileDescriptorProto, len(req.ProtoFile))
	copy(fds.File, req.ProtoFile)
	// protoc writes the output files directly, unlike the
	// protoc-gen-* plugin generators.
	// As such, we need to give it the right basenames and output
	// directory, so that it writes the files in the right place.
	for i, pf := range fds.File {
		if pf.GetName() == ftg {
			// Make a copy, to not modify the files for
			// other generators too.
			pf2 := *pf
			pf2.Name = proto.String(basename)
			fds.File[i] = &pf2
		}
	}
	// Because we merge all .gunk files into one 'all.proto' file,
	// we can use that package path on disk as the default location
	// to output generated files.
	gpkg, ok := g.findPkg(mainPkgPath)
	if !ok {
		return fmt.Errorf("failed to get package %s to protoc generate", mainPkgPath)
	}
	buf, err := protoutil.MarshalDeterministic(fds)
	if err != nil {
		return fmt.Errorf("cannot marshal deterministically: %w", err)
	}
	// output dir
	protocOutputPath := gpkg.Dir
	outDir, err := outPath(gen, protocOutputPath, mainPkg.Name)
	if err != nil {
		return fmt.Errorf("unable to build output path for %q: %w", gpkg.Dir, err)
	}
	if outDir != "" {
		if err := mkdirAll(outDir); err != nil {
			return fmt.Errorf("unable to create directory %q: %w", outDir, err)
		}
	}
	// Build up the protoc command line arguments.
	param := paramStringWithOut(gen, outDir)
	args := []string{
		fmt.Sprintf("--%s_out=%s", gen.ProtocGen, param),
		"--descriptor_set_in=/dev/stdin",
		basename,
	}
	var d *dirchanges.Watcher
	// if we have postproc - try to watch for new files (ignore otherwise)
	// unfortunately, protoc gives us no hint of what files it generated
	// so we look for FS changes
	if gen.HasPostproc() {
		d = dirchanges.New()
		if err := d.AddRecursive(protocOutputPath); err != nil {
			return err
		}
		d.FilterOps(dirchanges.Write, dirchanges.Move, dirchanges.Rename, dirchanges.Create)
	}
	cmd := log.ExecCommand(protocCommandPath, args...)
	cmd.Stdin = bytes.NewReader(buf)
	if _, err := cmd.Output(); err != nil {
		// TODO: For now, output the command name directly as
		// we actually use the /path/to/protoc when executing
		// the command, but this gives slightly uglier error
		// messages. Not sure what is best to do here, but
		// it should be consistent with running protoc-gen-*
		// errors (which currently don't use the /path/to/protoc-gen).
		return log.ExecError("protoc", err)
	}
	if gen.HasPostproc() {
		ev, err := d.Diff()
		if err != nil {
			return fmt.Errorf("file diff error: %w", err)
		}
		for _, ev := range ev {
			if !ev.IsDir() {
				bs, err := ioutil.ReadFile(ev.Path)
				if err != nil {
					return fmt.Errorf("failed to read file from %s for post processing: %w", ev.Path, err)
				}
				var nbs []byte
				if nbs, err = postProcess(bs, gen, mainPkgPath, g.gunkPkgs); err != nil {
					return fmt.Errorf("failed to execute post processing: %w", err)
				}
				if err := ioutil.WriteFile(ev.Path, nbs, ev.Mode()); err != nil {
					return fmt.Errorf("failed to write to file: %w", err)
				}
			}
		}
	}
	return nil
}

// generatePlugin invokes the specified binary in the config with the package
// requested in CodeGeneratorRequest. It expects exactly one file to be
// requested in CodeGeneratorRequest.
func (g *Generator) generatePlugin(req *pluginpb.CodeGeneratorRequest, gen configWithBinary) error {
	// Due to problems with some generators (grpc-gateway),
	// we need to ensure we either send a non-empty string or nil.
	if ps := gen.ParamString(); ps == "" {
		req.Parameter = nil
	} else {
		req.Parameter = proto.String(ps)
	}
	bs, err := protoutil.MarshalDeterministic(req)
	if err != nil {
		return fmt.Errorf("cannot marshal deterministically: %w", err)
	}
	cmd := log.ExecCommand(gen.actualCommand())
	cmd.Stdin = bytes.NewReader(bs)
	out, err := cmd.Output()
	if err != nil {
		return log.ExecError(gen.actualCommand(), err)
	}
	var resp pluginpb.CodeGeneratorResponse
	if err = proto.Unmarshal(out, &resp); err != nil {
		return err
	}
	if rerr := resp.GetError(); rerr != "" {
		return fmt.Errorf("error from generator %s: %s", gen.Command, rerr)
	}
	ftgs := req.GetFileToGenerate()
	var outputPath, mainPkgName, mainPkgPath string
	switch len(ftgs) {
	case 0:
		return fmt.Errorf("unexpected length of fileToGenerate, expected at least 1, got 0")
	case 1:
		ftg := ftgs[0]
		mainPkgPath, _ = filepath.Split(ftg)
		mainPkgPath = filepath.Clean(mainPkgPath)
		mainPkg, ok := g.gunkPkgs[mainPkgPath]
		if !ok {
			return fmt.Errorf("failed to get main package: %s", mainPkgPath)
		}
		mainPkgName = mainPkg.Name
		outputPath, err = outPath(gen.Generator, mainPkg.Dir, mainPkg.Name)
		if err != nil {
			return fmt.Errorf("failed to build path for %s: %w", mainPkg.Name, err)
		}
	default:
		// generate_single mode.
		if gen.Out == "" {
			return fmt.Errorf("output path must be specified for generate_single mode")
		}
		// mainPkgName should not be relied upon but set to not leave it empty.
		mainPkgName = "generate_single"
		outputPath, err = outPath(gen.Generator, "", mainPkgName)
		if err != nil {
			return fmt.Errorf("failed to build path for generate_single: %w", err)
		}
	}
	for _, rf := range resp.File {
		// Turn the relative package file path to the absolute
		// on-disk file path.
		// some code generators (go) return path with the full package path,
		// some (java-grpc) return just local path relative
		pkgPath, basename := filepath.Split(*rf.Name)
		pkgPath = filepath.Clean(pkgPath) // to remove trailing slashes

		var dir string

		_, isGunkPkg := g.gunkPkgs[pkgPath]
		if !isGunkPkg {
			// Use the longest prefix match if it's not found in gunkPkgs.
			matching := ""
			for path := range g.gunkPkgs {
				if strings.HasPrefix(pkgPath, path) {
					if len(path) > len(matching) {
						matching = path
						subdir := strings.TrimPrefix(pkgPath, path)
						dir = outputPath + subdir
					}
				}
			}

			// If there is no prefix match, it's likely a local relative path.
			if matching == "" {
				dir = outputPath
			}
		} else {
			dir = outputPath
		}
		data := []byte(*rf.Content)
		if gen.HasPostproc() {
			if mainPkgPath == "" {
				return fmt.Errorf("cannot run postprocessing in generate_single mode")
			}
			if data, err = postProcess(data, gen.Generator, mainPkgPath, g.gunkPkgs); err != nil {
				return fmt.Errorf("failed to execute post processing: %w", err)
			}
		}

		outPath := filepath.Join(dir, basename)
		if !isGunkPkg {
			outPath = filepath.Join(dir, *rf.Name)
		}

		// remove fake path
		outPath = strings.TrimPrefix(outPath, "fake-path.com/command-line-arguments/")

		outPath, err = pkgTpl(outPath, mainPkgName)
		if err != nil {
			return fmt.Errorf("unable to build output path for %q: %w", outPath, err)
		}

		// create path if not exists
		if outDir, _ := filepath.Split(outPath); outDir != "" {
			if err := mkdirAll(outDir); err != nil {
				return fmt.Errorf("unable to create directory %q: %w", outDir, err)
			}
		}

		if err := writeFile(outPath, data); err != nil {
			return fmt.Errorf("unable to write to file %q: %w", outPath, err)
		}
	}
	return nil
}

// newCodeGenRequest returns a CodeGeneratorRequest for the specified packages
// which requests generation for the packages and specifies the dependencies of
// the packages.
func (g *Generator) newCodeGenRequest(pkgPaths ...string) *pluginpb.CodeGeneratorRequest {
	req := &pluginpb.CodeGeneratorRequest{}
	for _, pkgPath := range pkgPaths {
		req.FileToGenerate = append(req.FileToGenerate, unifiedProtoFile(pkgPath))
	}
	for _, pfile := range g.allProto {
		req.ProtoFile = append(req.ProtoFile, pfile)
	}
	// ProtoFile must be sorted in topological order, so that each file's
	// dependencies are satisfied by previous files. This is a requirement
	// of some generators.
	req.ProtoFile = topologicalSort(req.ProtoFile)
	return req
}

// topologicalSort sorts a number of protobuf descriptor files so that each
// file's dependencies can be satisfied by previous files in the list. In other
// words, it sorts the files incrementally by their dependencies.
//
// The algorithm isn't optimal, as it is a form of quadratic insertion sort with
// the help of a map. However, we won't be dealing with large numbers of proto
// files as each Gunk package is a single "all.proto" file, so this will likely
// be enough for a while. The advantage is that the implementation is very
// simple.
func topologicalSort(files []*descriptorpb.FileDescriptorProto) []*descriptorpb.FileDescriptorProto {
	previous := make(map[string]bool)
	result := make([]*descriptorpb.FileDescriptorProto, 0, len(files))
_addLoop:
	for len(result) < len(files) {
	_fileLoop:
		for _, pfile := range files {
			name := *pfile.Name
			if previous[name] {
				// Already part of the result.
				continue
			}
			for _, dep := range pfile.Dependency {
				if !previous[dep] {
					// Depends on files not in result yet.
					continue _fileLoop
				}
			}
			// Add this file.
			previous[name] = true
			result = append(result, pfile)
			continue _addLoop
		}
		// We didn't find a file we could add.
		panic("could not sort proto files by dependencies. dependency cycle?")
	}
	return result
}

// translatePkg translates all the gunk files in a gunk package to the
// proto language. All the files within the package, including all the
// files for its transitive dependencies, must already be loaded.
func (g *Generator) translatePkg(pkgPath string) error {
	gpkg, ok := g.gunkPkgs[pkgPath]
	if !ok {
		return fmt.Errorf("failed to get package %s to translate", pkgPath)
	}
	pfilename := unifiedProtoFile(gpkg.PkgPath)
	if _, ok := g.allProto[pfilename]; ok {
		// Already translated, e.g. as a dependency.
		return nil
	}
	// Get file options for package
	fo, err := fileOptions(gpkg)
	if err != nil {
		return fmt.Errorf("unable to get file options: %v", err)
	}
	g.curPkg = gpkg
	g.usedImports = make(map[string]bool)

	protoGoPkgPath := pkgPath
	if pkgPath == "command-line-arguments" {
		// go compiler complains about missing slash in package path
		protoGoPkgPath = "fake-path.com/command-line-arguments"
	}

	// Set the GoPackage file option to be the gunk package name.
	fo.GoPackage = proto.String(protoGoPkgPath + ";" + gpkg.Name)

	// note - do not set above to gpkg.PkgPath or basename of that;
	// gunk files can have different names than path
	// (package github.com/foo/bar can be "package foobar").
	// We need to use "foobar", otherwise gunk will break
	// (not matching package paths)
	g.pfile = &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String(pfilename),
		Package: proto.String(gpkg.ProtoName),
		Options: fo,
	}
	g.allProto[pfilename] = g.pfile
	g.messageIndex = 0
	g.serviceIndex = 0
	g.enumIndex = 0
	for i, fpath := range gpkg.GunkNames {
		if err := g.appendFile(fpath, gpkg.GunkSyntax[i]); err != nil {
			return fmt.Errorf("%s: %v", g.Fset.Position(g.curPos), err)
		}
	}
	var leftToTranslate []string
	for _, gfile := range gpkg.GunkSyntax {
		for _, imp := range gfile.Imports {
			if imp.Name != nil && imp.Name.Name == "_" {
				// An underscore import.
				continue
			}
			opath, _ := strconv.Unquote(imp.Path.Value)
			pkg := g.gunkPkgs[opath]
			if pkg == nil || len(pkg.GunkNames) == 0 {
				// Not a gunk package, so no joint proto file to
				// depend on.
				continue
			}
			if !g.usedImports[opath] {
				// Only include imports that are used.
				continue
			}
			pfile := unifiedProtoFile(opath)
			if _, ok := g.allProto[pfile]; !ok {
				leftToTranslate = append(leftToTranslate, opath)
			}
			g.addProtoDep(pfile)
		}
	}
	// Do the recursive translatePkg calls at the end, since the generator
	// holds the state for the current package.
	for _, pkgPath := range leftToTranslate {
		if err := g.translatePkg(pkgPath); err != nil {
			return err
		}
	}
	return nil
}

// fileOptions will return the proto file options that have been set in the
// gunk package. These include "JavaPackage", "Deprecated", "PhpNamespace", etc.
func fileOptions(pkg *loader.GunkPackage) (*descriptorpb.FileOptions, error) {
	fo := &descriptorpb.FileOptions{}
	for _, f := range pkg.GunkSyntax {
		for _, tag := range pkg.GunkTags[f] {
			switch s := tag.Type.String(); s {
			case "github.com/gunk/opt/proto.Package":
				pkg.ProtoName = constant.StringVal(tag.Value)
			case "github.com/gunk/opt/file.OptimizeFor":
				oValue := descriptorpb.FileOptions_OptimizeMode(protoEnumValue(tag.Value))
				fo.OptimizeFor = &oValue
			case "github.com/gunk/opt/file.Deprecated":
				fo.Deprecated = proto.Bool(constant.BoolVal(tag.Value))
			// Java package options.
			case "github.com/gunk/opt/file/java.Package":
				fo.JavaPackage = proto.String(constant.StringVal(tag.Value))
			case "github.com/gunk/opt/file/java.OuterClassname":
				fo.JavaOuterClassname = proto.String(constant.StringVal(tag.Value))
			case "github.com/gunk/opt/file/java.MultipleFiles":
				fo.JavaMultipleFiles = proto.Bool(constant.BoolVal(tag.Value))
			case "github.com/gunk/opt/file/java.StringCheckUtf8":
				fo.JavaStringCheckUtf8 = proto.Bool(constant.BoolVal(tag.Value))
			case "github.com/gunk/opt/file/java.GenericServices":
				fo.JavaGenericServices = proto.Bool(constant.BoolVal(tag.Value))
			// Swift package options.
			case "github.com/gunk/opt/file/swift.Prefix":
				fo.SwiftPrefix = proto.String(constant.StringVal(tag.Value))
			// Ruby package options.
			case "github.com/gunk/opt/file/ruby.Package":
				fo.RubyPackage = proto.String(constant.StringVal(tag.Value))
			// CSharp package options.
			case "github.com/gunk/opt/file/csharp.Namespace":
				fo.CsharpNamespace = proto.String(constant.StringVal(tag.Value))
			// ObjectiveC package options.
			case "github.com/gunk/opt/file/objc.ClassPrefix":
				fo.ObjcClassPrefix = proto.String(constant.StringVal(tag.Value))
			// PHP package options.
			case "github.com/gunk/opt/file/php.Namespace":
				fo.PhpNamespace = proto.String(constant.StringVal(tag.Value))
			case "github.com/gunk/opt/file/php.ClassPrefix":
				fo.PhpClassPrefix = proto.String(constant.StringVal(tag.Value))
			case "github.com/gunk/opt/file/php.MetadataNamespace":
				fo.PhpMetadataNamespace = proto.String(constant.StringVal(tag.Value))
			case "github.com/gunk/opt/file/php.GenericServices":
				fo.PhpGenericServices = proto.Bool(constant.BoolVal(tag.Value))
			case "github.com/gunk/opt/openapiv2.Swagger":
				o := &options.Swagger{}
				reflectutil.UnmarshalAST(o, tag.Expr)
				proto.SetExtension(fo, options.E_Openapiv2Swagger, o)
			case "github.com/gunk/opt/xo.SkipPrefix":
				o := &xo.FileOverride{}
				o.SkipPrefix = constant.BoolVal(tag.Value)
				proto.SetExtension(fo, xo.E_FileOverrides, o)
			default:
				return nil, fmt.Errorf("gunk package option %q not supported", s)
			}
		}
	}
	// Set unset protocol buffer fields to their default values.
	reflectutil.SetDefaults(fo)
	return fo, nil
}

// appendFile translates a single gunk file to protobuf, appending its contents
// to the package's proto file.
func (g *Generator) appendFile(fpath string, file *ast.File) error {
	if _, ok := g.allProto[fpath]; ok {
		// already translated
		return nil
	}
	g.gfile = file

	if g.pfile.SourceCodeInfo == nil {
		g.pfile.SourceCodeInfo = &descriptorpb.SourceCodeInfo{}
	}

	g.addDoc(file.Doc.Text(), packagePath)
	for _, decl := range file.Decls {
		g.curPos = decl.Pos()
		if err := g.translateDecl(decl); err != nil {
			return err
		}
	}
	return nil
}

// translateDecl translates a top-level declaration in a gunk file. It
// only acts on type declarations; struct types become proto messages,
// interfaces become services, and basic integer types become enums.
func (g *Generator) translateDecl(decl ast.Decl) error {
	gd, ok := decl.(*ast.GenDecl)
	if !ok {
		return fmt.Errorf("invalid declaration %T", decl)
	}
	switch gd.Tok {
	case token.TYPE:
		// continue below
	case token.CONST:
		return nil // used for enums
	case token.IMPORT:
		return nil // imports; ignore
	default:
		return fmt.Errorf("invalid declaration token %v", gd.Tok)
	}
	for _, spec := range gd.Specs {
		ts := spec.(*ast.TypeSpec)
		g.curPos = ts.Pos()
		switch ts.Type.(type) {
		case *ast.StructType:
			msg, err := g.convertMessage(ts)
			if err != nil {
				return err
			}
			g.pfile.MessageType = append(g.pfile.MessageType, msg)
		case *ast.InterfaceType:
			srv, err := g.convertService(ts)
			if err != nil {
				return err
			}
			g.pfile.Service = append(g.pfile.Service, srv)
		case *ast.Ident:
			enum, err := g.convertEnum(ts)
			if err != nil {
				return err
			}
			// This can happen if the enum has no values.
			if enum != nil {
				g.pfile.EnumType = append(g.pfile.EnumType, enum)
			}
		default:
			return fmt.Errorf("invalid declaration type %T", ts.Type)
		}
	}
	return nil
}

// addDoc inserts the provided documentation text into protobuf with its path
// after formatting it into the format proto requires.
func (g *Generator) addDoc(text string, path ...int32) {
	if text == "" {
		return
	}
	// go's ast.TypeSpec.Doc.Text() trims left-trailing spaces on each line of multi-line comment,
	// while proto's LeadingComments needs them
	//
	// block comments still look bad, but that's not a priority now
	lines := strings.Split(text, "\n")
	newText := " " + strings.Join(lines, "\n ")
	newText = strings.TrimRight(newText, " \n")

	g.pfile.SourceCodeInfo.Location = append(g.pfile.SourceCodeInfo.Location,
		&descriptorpb.SourceCodeInfo_Location{
			Path:            path,
			LeadingComments: &newText,
			// just add some nonsense to satisfy protoc-gen-go
			Span: []int32{1, 2, 3},
		},
	)
}

// messageOptions returns the MessageOptions set using Gunk tags.
func (g *Generator) messageOptions(tspec *ast.TypeSpec) (*descriptorpb.MessageOptions, error) {
	o := &descriptorpb.MessageOptions{}
	xoOpts := &xo.MessageOverride{}
	var xoOk bool
	// Check message tags
	for _, tag := range g.curPkg.GunkTags[tspec] {
		switch s := tag.Type.String(); s {
		case "github.com/gunk/opt/message.MessageSetWireFormat":
			o.MessageSetWireFormat = proto.Bool(constant.BoolVal(tag.Value))
		case "github.com/gunk/opt/message.NoStandardDescriptorAccessor":
			o.NoStandardDescriptorAccessor = proto.Bool(constant.BoolVal(tag.Value))
		case "github.com/gunk/opt/message.Deprecated":
			o.Deprecated = proto.Bool(constant.BoolVal(tag.Value))
		case "github.com/gunk/opt/openapiv2.Schema":
			schema := &options.Schema{}
			reflectutil.UnmarshalAST(schema, tag.Expr)
			proto.SetExtension(o, options.E_Openapiv2Schema, schema)
		case "github.com/gunk/opt/xo.Manual":
			xoOpts.Manual, xoOk = constant.BoolVal(tag.Value), true
		case "github.com/gunk/opt/xo.Ignore":
			xoOpts.Ignore, xoOk = constant.BoolVal(tag.Value), true
		case "github.com/gunk/opt/xo.Embed":
			xoOpts.EmbedAsJson, xoOk = constant.BoolVal(tag.Value), true
		case "github.com/gunk/opt/xo.HasMany":
			otmEntry := &xo.OneToMany{}
			reflectutil.UnmarshalAST(otmEntry, tag.Expr)
			xoOpts.HasMany = append(xoOpts.HasMany, otmEntry)
			xoOk = true
		default:
			return nil, fmt.Errorf("gunk message option %q not supported", s)
		}
	}
	if xoOk {
		proto.SetExtension(o, xo.E_MsgOverrides, xoOpts)
		g.addProtoDep("xo/xo.proto")
	}
	reflectutil.SetDefaults(o)
	return o, nil
}

// FieldOptions returns the FieldOptions set using Gunk tags.
func (g *Generator) fieldOptions(field *ast.Field) (*descriptorpb.FieldOptions, error) {
	o := &descriptorpb.FieldOptions{}
	xoOpts := &xo.FieldOverride{}
	var xoOk bool
	// Check field tags
	for _, tag := range g.curPkg.GunkTags[field] {
		switch s := tag.Type.String(); s {
		case "github.com/gunk/opt/field.Packed":
			o.Packed = proto.Bool(constant.BoolVal(tag.Value))
		case "github.com/gunk/opt/field.Lazy":
			o.Lazy = proto.Bool(constant.BoolVal(tag.Value))
		case "github.com/gunk/opt/field.Deprecated":
			o.Deprecated = proto.Bool(constant.BoolVal(tag.Value))
		case "github.com/gunk/opt/field/cc.Type":
			oValue := descriptorpb.FieldOptions_CType(protoEnumValue(tag.Value))
			o.Ctype = &oValue
		case "github.com/gunk/opt/field/js.Type":
			oValue := descriptorpb.FieldOptions_JSType(protoEnumValue(tag.Value))
			o.Jstype = &oValue
		case "github.com/gunk/opt/openapiv2.Schema":
			for _, elt := range tag.Expr.(*ast.CompositeLit).Elts {
				kv := elt.(*ast.KeyValueExpr)
				switch kv.Key.(*ast.Ident).Name {
				case "JSONSchema":
					jsonSchema := &options.JSONSchema{}
					reflectutil.UnmarshalAST(jsonSchema, kv.Value)
					proto.SetExtension(o, options.E_Openapiv2Field, jsonSchema)
				}
			}
		case "github.com/gunk/opt/openapiv2.JSONSchema":
			jsonSchema := &options.JSONSchema{}
			reflectutil.UnmarshalAST(jsonSchema, tag.Expr)
			proto.SetExtension(o, options.E_Openapiv2Field, jsonSchema)
		case "github.com/gunk/opt/xo.IndexType":
			xoOk = true
			switch v, _ := constant.Uint64Val(tag.Value); v {
			case 0:
				xoOpts.Index = xo.FieldOverride_INDEX
			case 1:
				xoOpts.Index = xo.FieldOverride_UNIQUE
			default:
				return nil, fmt.Errorf("unknown value for xo.IndexType: %d", v)
			}
		case "github.com/gunk/opt/xo.Ignore":
			xoOpts.Ignore, xoOk = constant.BoolVal(tag.Value), true
		case "github.com/gunk/opt/xo.Embed":
			xoOpts.EmbedAsJson, xoOk = constant.BoolVal(tag.Value), true
		case "github.com/gunk/opt/xo.Default":
			xoOpts.DefaultValue, xoOk = constant.StringVal(tag.Value), true
		case "github.com/gunk/opt/xo.Ref":
			ref := &xo.Ref{}
			reflectutil.UnmarshalAST(ref, tag.Expr)
			xoOpts.Ref, xoOk = ref, true
		case "github.com/gunk/opt/xo.Nullable":
			xoOpts.Nullable, xoOk = constant.BoolVal(tag.Value), true
		default:
			return nil, fmt.Errorf("gunk field option %q not supported", s)
		}
	}
	if xoOk {
		proto.SetExtension(o, xo.E_FieldOverrides, xoOpts)
		g.addProtoDep("xo/xo.proto")
	}
	reflectutil.SetDefaults(o)
	return o, nil
}

// convertMessage converts the provided type spec of a struct into a descriptor
// that describes a message.
func (g *Generator) convertMessage(tspec *ast.TypeSpec) (*descriptorpb.DescriptorProto, error) {
	g.addDoc(tspec.Doc.Text(), messagePath, g.messageIndex)
	msg := &descriptorpb.DescriptorProto{
		Name: proto.String(tspec.Name.Name),
	}
	messageOptions, err := g.messageOptions(tspec)
	if err != nil {
		return nil, fmt.Errorf("error getting message options: %v", err)
	}
	msg.Options = messageOptions
	stype := tspec.Type.(*ast.StructType)
	for i, field := range stype.Fields.List {
		if len(field.Names) != 1 {
			return nil, fmt.Errorf("fields must have exactly one name")
		}
		fieldName := field.Names[0].Name
		g.addDoc(field.Doc.Text(), messagePath, g.messageIndex, messageFieldPath, int32(i))
		ftype := g.curPkg.TypesInfo.TypeOf(field.Type)
		g.curPos = field.Pos()
		var ptype descriptorpb.FieldDescriptorProto_Type
		var plabel descriptorpb.FieldDescriptorProto_Label
		var tname string
		var msgNestedType *descriptorpb.DescriptorProto
		// Check to see if the type is a map. Maps need to be made into a
		// repeated nested message containing key and value fields.
		if mtype, ok := ftype.(*types.Map); ok {
			ptype = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
			plabel = descriptorpb.FieldDescriptorProto_LABEL_REPEATED
			var err error
			tname, msgNestedType, err = g.convertMap(tspec.Name.Name, fieldName, mtype)
			if err != nil {
				return nil, err
			}
			msg.NestedType = append(msg.NestedType, msgNestedType)
		} else {
			var err error
			ptype, plabel, tname, err = g.convertType(ftype)
			if err != nil {
				return nil, err
			}
		}
		if ptype == 0 {
			return nil, fmt.Errorf("unsupported field type: %v", ftype)
		}
		// Check that the struct field has a tag. We currently
		// require all struct fields to have a tag; this is used
		// to assign the position number for a field, ie: `pb:"1"`
		if field.Tag == nil {
			return nil, fmt.Errorf("missing required tag on %s", fieldName)
		}
		// Can skip the error here because we've already parsed the file.
		str, _ := strconv.Unquote(field.Tag.Value)
		tag := reflect.StructTag(str)
		// TODO: record the position numbers used so we can return an
		// error if position number is used more than once? This would
		// also allow us to automatically assign fields a position
		// number if it is missing one.
		num, err := protoNumber(tag)
		if err != nil {
			return nil, fmt.Errorf("unable to convert tag to number on %s: %v", fieldName, err)
		}
		fieldOptions, err := g.fieldOptions(field)
		if err != nil {
			return nil, fmt.Errorf("error getting field options: %v", err)
		}
		msg.Field = append(msg.Field, &descriptorpb.FieldDescriptorProto{
			Name:     proto.String(fieldName),
			Number:   num,
			TypeName: protoStringOrNil(tname),
			Type:     &ptype,
			Label:    &plabel,
			JsonName: jsonName(tag),
			Options:  fieldOptions,
		})
	}
	g.messageIndex++
	return msg, nil
}

// serviceOptions returns the ServiceOptions set using Gunk tags.
func (g *Generator) serviceOptions(tspec *ast.TypeSpec) (*descriptorpb.ServiceOptions, error) {
	o := &descriptorpb.ServiceOptions{}
	for _, tag := range g.curPkg.GunkTags[tspec] {
		switch s := tag.Type.String(); s {
		case "github.com/gunk/opt/service.Deprecated":
			o.Deprecated = proto.Bool(constant.BoolVal(tag.Value))
		default:
			return nil, fmt.Errorf("gunk service option %q not supported", s)
		}
	}
	reflectutil.SetDefaults(o)
	return o, nil
}

// methodOptions returns the MethodOptions set using Gunk tags.
func (g *Generator) methodOptions(method *ast.Field) (*descriptorpb.MethodOptions, error) {
	o := &descriptorpb.MethodOptions{}
	var httpRule *annotations.HttpRule
	for _, tag := range g.curPkg.GunkTags[method] {
		switch s := tag.Type.String(); s {
		case "github.com/gunk/opt/method.Deprecated":
			o.Deprecated = proto.Bool(constant.BoolVal(tag.Value))
		case "github.com/gunk/opt/method.IdempotencyLevel":
			oValue := descriptorpb.MethodOptions_IdempotencyLevel(protoEnumValue(tag.Value))
			o.IdempotencyLevel = &oValue
		case "github.com/gunk/opt/http.Match":
			// Capture the values required to use in annotations.HttpRule.
			// We need to evaluate the entire expression, and then we can
			// create an annotations.HttpRule.
			var path string
			var body string
			method := "GET"
			for _, elt := range tag.Expr.(*ast.CompositeLit).Elts {
				kv := elt.(*ast.KeyValueExpr)
				val, _ := strconv.Unquote(kv.Value.(*ast.BasicLit).Value)
				switch name := kv.Key.(*ast.Ident).Name; name {
				case "Method":
					method = val
				case "Path":
					// TODO: grpc-gateway doesn't allow paths with a trailing "/", should
					// we return an error here, because the error from grpc-gateway is very
					// cryptic and unhelpful?
					// https://github.com/grpc-ecosystem/grpc-gateway/issues/472
					path = val
				case "Body":
					body = val
				default:
					return nil, fmt.Errorf("unknown expression key %q", name)
				}
			}
			rule := &annotations.HttpRule{
				Body: body,
			}
			if httpRule == nil {
				httpRule = rule
			} else {
				httpRule.AdditionalBindings = append(httpRule.AdditionalBindings, rule)
			}
			switch method {
			case "GET":
				rule.Pattern = &annotations.HttpRule_Get{Get: path}
			case "POST":
				rule.Pattern = &annotations.HttpRule_Post{Post: path}
			case "DELETE":
				rule.Pattern = &annotations.HttpRule_Delete{Delete: path}
			case "PUT":
				rule.Pattern = &annotations.HttpRule_Put{Put: path}
			case "PATCH":
				rule.Pattern = &annotations.HttpRule_Patch{Patch: path}
			default:
				return nil, fmt.Errorf("unknown method type: %q", method)
			}
		case "github.com/gunk/opt/openapiv2.Operation":
			op := &options.Operation{}
			reflectutil.UnmarshalAST(op, tag.Expr)
			proto.SetExtension(o, options.E_Openapiv2Operation, op)
			g.addProtoDep("protoc-gen-openapiv2/options/annotations.proto")
		default:
			return nil, fmt.Errorf("gunk method option %q not supported", s)
		}
	}
	if httpRule != nil {
		proto.SetExtension(o, annotations.E_Http, httpRule)
		g.addProtoDep("google/api/annotations.proto")
	}
	reflectutil.SetDefaults(o)
	return o, nil
}

func (g *Generator) convertService(tspec *ast.TypeSpec) (*descriptorpb.ServiceDescriptorProto, error) {
	srv := &descriptorpb.ServiceDescriptorProto{
		Name: proto.String(tspec.Name.Name),
	}
	serviceOptions, err := g.serviceOptions(tspec)
	if err != nil {
		return nil, fmt.Errorf("error getting service options: %v", err)
	}
	srv.Options = serviceOptions
	itype := tspec.Type.(*ast.InterfaceType)
	for i, method := range itype.Methods.List {
		if len(method.Names) != 1 {
			return nil, fmt.Errorf("methods must have exactly one name")
		}
		g.addDoc(method.Doc.Text(), servicePath, g.serviceIndex, serviceMethodPath, int32(i))
		g.curPos = method.Pos()
		pmethod := &descriptorpb.MethodDescriptorProto{
			Name: proto.String(method.Names[0].Name),
		}
		methodOptions, err := g.methodOptions(method)
		if err != nil {
			return nil, fmt.Errorf("error getting method options: %v", err)
		}
		pmethod.Options = methodOptions
		sign := g.curPkg.TypesInfo.TypeOf(method.Type).(*types.Signature)
		pmethod.InputType, pmethod.ClientStreaming, err = g.convertParameter(sign.Params())
		if err != nil {
			return nil, err
		}
		pmethod.OutputType, pmethod.ServerStreaming, err = g.convertParameter(sign.Results())
		if err != nil {
			return nil, err
		}
		srv.Method = append(srv.Method, pmethod)
	}
	g.serviceIndex++
	return srv, nil
}

// convertMap will translate a Go map to a Protobuf respresentation of a map,
// returning the nested type name and definition.
//
// Protobuf represents a map as a nested message on the parent message. This
// nested message contains two fields; key and value (map[key]value), and has
// the MapEntry option set to true.
//
// https://developers.google.com/protocol-buffers/docs/proto#maps
func (g *Generator) convertMap(parentName, fieldName string, mapTyp *types.Map) (string, *descriptorpb.DescriptorProto, error) {
	mapName := fieldName + "Entry"
	typeName, err := g.qualifiedTypeName(parentName+"."+mapName, nil)
	if err != nil {
		return "", nil, err
	}
	keyType, _, keyTypeName, err := g.convertType(mapTyp.Key())
	if err != nil {
		return "", nil, err
	}
	if keyType == 0 {
		return "", nil, nil
	}
	elemType, _, elemTypeName, err := g.convertType(mapTyp.Elem())
	if err != nil {
		return "", nil, err
	}
	if elemType == 0 {
		return "", nil, nil
	}
	fieldLabel := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
	nestedType := &descriptorpb.DescriptorProto{
		Name: proto.String(mapName),
		Options: &descriptorpb.MessageOptions{
			MapEntry: proto.Bool(true),
		},
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:     proto.String("key"),
				Number:   proto.Int32(1),
				Label:    &fieldLabel,
				Type:     &keyType,
				TypeName: protoStringOrNil(keyTypeName),
			},
			{
				Name:     proto.String("value"),
				Number:   proto.Int32(2),
				Label:    &fieldLabel,
				Type:     &elemType,
				TypeName: protoStringOrNil(elemTypeName),
			},
		},
	}
	return typeName, nestedType, nil
}

// convertParameter converts the provided parameters to their corresponding
// types and returns if the parameter is a stream (channel).
func (g *Generator) convertParameter(tuple *types.Tuple) (*string, *bool, error) {
	switch tuple.Len() {
	case 0:
		g.addProtoDep("google/protobuf/empty.proto")
		return proto.String(".google.protobuf.Empty"), nil, nil
	case 1:
		// below
	default:
		return nil, nil, fmt.Errorf("multiple parameters are not supported")
	}
	param := tuple.At(0).Type()
	_, label, tname, err := g.convertType(param)
	if err != nil {
		return nil, nil, err
	}
	if tname == "" {
		return nil, nil, fmt.Errorf("unsupported parameter type: %v", param)
	}
	if label == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
		return nil, nil, fmt.Errorf("parameter type should not be repeated")
	}
	isStream := proto.Bool(false)
	if _, ok := param.(*types.Chan); ok {
		isStream = proto.Bool(true)
	}
	return &tname, isStream, nil
}

// enumOptions returns the EnumOptions set using Gunk tags.
func (g *Generator) enumOptions(tspec *ast.TypeSpec) (*descriptorpb.EnumOptions, error) {
	o := &descriptorpb.EnumOptions{}
	for _, tag := range g.curPkg.GunkTags[tspec] {
		switch s := tag.Type.String(); s {
		case "github.com/gunk/opt/enum.AllowAlias":
			o.AllowAlias = proto.Bool(constant.BoolVal(tag.Value))
		case "github.com/gunk/opt/enum.Deprecated":
			o.Deprecated = proto.Bool(constant.BoolVal(tag.Value))
		default:
			return nil, fmt.Errorf("gunk enum option %q not supported", s)
		}
	}
	reflectutil.SetDefaults(o)
	return o, nil
}

// enumValueOptions returns the EnumValueOptions set using Gunk tags.
func (g *Generator) enumValueOptions(vspec *ast.ValueSpec) (*descriptorpb.EnumValueOptions, error) {
	o := &descriptorpb.EnumValueOptions{}
	for _, tag := range g.curPkg.GunkTags[vspec] {
		switch s := tag.Type.String(); s {
		case "github.com/gunk/opt/enumvalues.Deprecated":
			o.Deprecated = proto.Bool(constant.BoolVal(tag.Value))
		default:
			return nil, fmt.Errorf("gunk enumvalue option %q not supported", s)
		}
	}
	reflectutil.SetDefaults(o)
	return o, nil
}

// convertEnum converts the provided const TypeSpec to an EnumDescriptorProto.
// It returns (nil, nil) if there are no values for the enum type.
func (g *Generator) convertEnum(tspec *ast.TypeSpec) (*descriptorpb.EnumDescriptorProto, error) {
	g.addDoc(tspec.Doc.Text(), enumPath, g.enumIndex)
	enum := &descriptorpb.EnumDescriptorProto{
		Name: proto.String(tspec.Name.Name),
	}
	enumOptions, err := g.enumOptions(tspec)
	if err != nil {
		return nil, fmt.Errorf("error getting enum options: %v", err)
	}
	enum.Options = enumOptions
	enumType := g.curPkg.TypesInfo.TypeOf(tspec.Name)
	for _, decl := range g.gfile.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for i, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			// .proto files have the same limitation, and it
			// allows per-value godocs
			if len(vs.Names) != 1 {
				return nil, fmt.Errorf("value specs must have exactly one name")
			}
			name := vs.Names[0]
			if g.curPkg.TypesInfo.TypeOf(name) != enumType {
				continue
			}
			g.curPos = vs.Pos()
			docText := vs.Doc.Text()

			switch {
			case docText == "":
				// The original comment only had gunk tags, and
				// no actual documentation for us to keep.
			case strings.HasPrefix(docText, name.Name):
				// SomeVal will be exported as SomeType_SomeVal
				docText = tspec.Name.Name + "_" + vs.Doc.Text()
				fallthrough
			default:
				g.addDoc(docText, enumPath, g.enumIndex,
					enumValuePath, int32(i))
			}
			val := g.curPkg.TypesInfo.Defs[name].(*types.Const).Val()
			ival, _ := constant.Int64Val(val)
			enumValueOptions, err := g.enumValueOptions(vs)
			if err != nil {
				return nil, fmt.Errorf("error getting enum value options: %v", err)
			}

			enum.Value = append(enum.Value, &descriptorpb.EnumValueDescriptorProto{
				Name:    proto.String(name.Name),
				Number:  proto.Int32(int32(ival)),
				Options: enumValueOptions,
			})
		}
	}
	g.enumIndex++
	// If an enum doesn't have any values
	if len(enum.Value) == 0 {
		return nil, nil
	}
	return enum, nil
}

// qualifiedTypeName will format the type name for that package. If the
// package is nil, it will format the type for the current package that is
// being processed.
//
// Currently we format the type as ".<pkg_name>.<type_name>"
func (g *Generator) qualifiedTypeName(typeName string, pkg *types.Package) (string, error) {
	// If pkg is nil, we should format the type for the current package.
	if pkg == nil {
		return "." + g.curPkg.ProtoName + "." + typeName, nil
	}
	gpkg, ok := g.gunkPkgs[pkg.Path()]
	if !ok {
		return "", fmt.Errorf("failed to get package %s to get qualified type name", pkg.Path())
	}
	return "." + gpkg.ProtoName + "." + typeName, nil
}

// convertType converts a Go field or parameter type to Protobuf, returning its
// type descriptor, a label such as "repeated", and a name, if the final type is
// an enum or a message.
func (g *Generator) convertType(typ types.Type) (descriptorpb.FieldDescriptorProto_Type, descriptorpb.FieldDescriptorProto_Label, string, error) {
	switch typ := typ.(type) {
	case *types.Chan:
		return g.convertType(typ.Elem())
	case *types.Basic:
		// Map Go types to proto types:
		// https://developers.google.com/protocol-buffers/docs/proto3#scalar
		switch typ.Kind() {
		case types.String:
			return descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, "", nil
		case types.Int, types.Int32:
			return descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, "", nil
		case types.Uint, types.Uint32:
			return descriptorpb.FieldDescriptorProto_TYPE_UINT32, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, "", nil
		case types.Int64:
			return descriptorpb.FieldDescriptorProto_TYPE_INT64, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, "", nil
		case types.Uint64:
			return descriptorpb.FieldDescriptorProto_TYPE_UINT64, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, "", nil
		case types.Float32:
			return descriptorpb.FieldDescriptorProto_TYPE_FLOAT, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, "", nil
		case types.Float64:
			return descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, "", nil
		case types.Bool:
			return descriptorpb.FieldDescriptorProto_TYPE_BOOL, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, "", nil
		}
	case *types.Named:
		switch typ.String() {
		case "time.Time":
			g.addProtoDep("google/protobuf/timestamp.proto")
			return descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".google.protobuf.Timestamp", nil
		case "time.Duration":
			g.addProtoDep("google/protobuf/duration.proto")
			return descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".google.protobuf.Duration", nil
		case "encoding/json.RawMessage":
			g.addProtoDep("google/protobuf/struct.proto")
			return descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".google.protobuf.Value", nil
		}
		fullName, err := g.qualifiedTypeName(typ.Obj().Name(), typ.Obj().Pkg())
		if err != nil {
			return 0, 0, "", err
		}
		g.usedImports[typ.Obj().Pkg().Path()] = true
		switch u := typ.Underlying().(type) {
		case *types.Basic:
			switch u.Kind() {
			case types.Int, types.Int32:
				return descriptorpb.FieldDescriptorProto_TYPE_ENUM, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, fullName, nil
			}
		case *types.Struct:
			return descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, fullName, nil
		}
	case *types.Slice:
		if eTyp, ok := typ.Elem().(*types.Basic); ok {
			if eTyp.Kind() == types.Byte {
				return descriptorpb.FieldDescriptorProto_TYPE_BYTES, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, "", nil
			}
		}
		dtyp, _, name, err := g.convertType(typ.Elem())
		if err != nil {
			return 0, 0, "", err
		}
		if dtyp == 0 {
			return 0, 0, "", nil
		}
		return dtyp, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, name, nil
	}
	return 0, 0, "", nil
}

// addProtoDep is called when a gunk file is known to require importing of a
// proto file, such as when using google.protobuf.Empty.
func (g *Generator) addProtoDep(protoPath string) {
	for _, dep := range g.pfile.Dependency {
		if dep == protoPath {
			return // already in there
		}
	}
	g.pfile.Dependency = append(g.pfile.Dependency, protoPath)
}

// loadProtoDeps loads all the missing proto dependencies added with
// addProtoDep.
func (g *Generator) loadProtoDeps() error {
	loaded := make(map[string]bool)
	var list []string
	for _, pfile := range g.allProto {
		for _, dep := range pfile.Dependency {
			if _, e := g.allProto[dep]; !e && !loaded[dep] {
				loaded[dep] = true
				list = append(list, dep)
			}
		}
	}
	files, err := g.protoLoader.LoadProto(list...)
	if err != nil {
		return err
	}
	for _, pfile := range files {
		g.allProto[*pfile.Name] = pfile
	}
	return nil
}

// writeFile writes a file.
func writeFile(path string, buf []byte) error {
	return ioutil.WriteFile(path, buf, 0o644)
}

// mkdirAll creates a directory.
func mkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}

// pkgTpl processes the provided package path as a template, replacing Package
// with the package name.
func pkgTpl(tmpl string, pkg string) (string, error) {
	if !strings.Contains(tmpl, "{{") {
		return tmpl, nil
	}
	// templated path
	tpl, err := template.New("path").Parse(tmpl)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	if err := tpl.Execute(buf, map[string]interface{}{
		"Package": pkg,
	}); err != nil {
		return "", err
	}
	return filepath.Clean(strings.TrimSpace(buf.String())), nil
}

// paramStringWithOut will return the generator paramaters formatted
// for protoc, including where protoc should output the generated files.
func paramStringWithOut(g config.Generator, outDir string) string {
	// If no out path was specified, use the package directory.
	if params := g.ParamString(); params != "" {
		return params + ":" + outDir
	}
	return outDir
}

// outPath determines the path for a generator to write generated files to. It
// will use 'packageDir' if no 'out' key was set in the config.
func outPath(g config.Generator, packageDir string, pkg string) (string, error) {
	if g.Out == "" {
		return packageDir, nil
	}
	out, err := pkgTpl(g.Out, pkg)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(g.Out) {
		return out, nil
	}
	return filepath.Join(g.ConfigDir, out), nil
}
