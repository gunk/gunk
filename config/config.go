package config

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kenshaw/ini"
	"github.com/kenshaw/ini/parser"
)

const (
	DefaultTag = "default"

	goModFilename = "go.mod"
	gitFilename   = ".git"
)

type KeyValue struct {
	Key   string
	Value string
}

type Generator struct {
	ProtocGen     string // The type of protoc generator that should be run; js, python, etc.
	Command       string
	PluginVersion string // we can pin a protoc-gen-XX version
	Params        []KeyValue
	ConfigDir     string
	Out           string
	JSONPostProc  bool
	FixPaths      bool
	Shortened     bool // only for `gunk vet`
	Single        bool
}

func (g Generator) IsDoc() bool {
	return g.Command == "doc"
}

func (g Generator) IsProtoc() bool {
	return g.ProtocGen != ""
}

func (g Generator) Code() string {
	if g.ProtocGen != "" {
		return g.ProtocGen
	}
	return strings.TrimPrefix(g.Command, "protoc-gen-")
}

func (g Generator) HasPostproc() bool {
	if g.Code() == "go" || g.Code() == "grpc-gateway" || g.Code() == "grpc-go" {
		// for gofumpt
		return true
	}
	return g.JSONPostProc || g.FixPaths
}

func (g Generator) GetParam(key string) (string, bool) {
	for _, p := range g.Params {
		if p.Key == key {
			return p.Value, true
		}
	}
	return "", false
}

func (g Generator) ParamString() string {
	params := make([]string, len(g.Params))
	for i, p := range g.Params {
		if p.Value != "" {
			params[i] = fmt.Sprintf("%s=%s", p.Key, p.Value)
		} else {
			params[i] = p.Key
		}
	}
	return strings.Join(params, ",")
}

type Config struct {
	Dir           string
	Out           string
	ImportPath    string
	ProtocPath    string
	ProtocVersion string
	Generators    []Generator
	Format        FormatConfig
	DocsConfig    map[string]*DocConfig
}

// FormatConfig is configuration for the format command.
type FormatConfig struct {
	// Whether to set all JSON fields to their "correct" names.
	JSON bool
	// Whether to set all PB fields to be ordered.
	PB bool
	// List of initialisms to use when formatting JSON.
	Initialisms []string
}

// DocConfig is configuration for the docs generation output
type DocConfig struct {
	// User-facing name of the tag.
	Name string
	// Path to a file that contains preamble markdown for section
	Preamble string
	// Weight is used when sorting the tags in the documentation.
	Weight int
	// List of packages part of this section. Can either just the full path to
	// the package, or just the package name.
	Packages []string
}

// Load will attempt to find the .gunkconfig in the 'dir', working
// its way up to each parent looking for a .gunkconfig. Currently,
// Load will only stop when it is unable to go any further up the
// directory structure or until it finds a 'go.mod' file, or a
// '.git' file or folder.
//
// Passing in an empty 'dir' will tell Load to look in the current
// working directory.
func Load(dir string) (*Config, error) {
	var err error
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("error getting working directory: %v", err)
		}
	}
	cfgs := []*Config{}
	for {
		configPath := filepath.Join(dir, ".gunkconfig")
		reader, err := os.Open(configPath)
		if err == nil {
			defer reader.Close()
			cfg, err := LoadSingle(reader, dir)
			if err != nil {
				return nil, fmt.Errorf("error loading %q: %v", configPath, err)
			}
			// Patch in the directory of where to output the generated
			// files. And patch in the 'out' path if it has been set globally,
			// and not in the generate section.
			for i, gen := range cfg.Generators {
				if cfg.Out != "" && gen.Out == "" {
					cfg.Generators[i].Out = cfg.Out
				}
			}
			cfgs = append(cfgs, cfg)
		}
		// Check to see if this directory contains a 'go.mod' file or '.git'
		// file or folder. If so, we assume that is the root of the project
		// and we have found all the gunk configs.
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("unable to list files in directory %q", dir)
		}
		foundProjectRoot := false
		for _, f := range files {
			if f.Name() == goModFilename || f.Name() == gitFilename {
				foundProjectRoot = true
				break
			}
		}
		if foundProjectRoot {
			break
		}
		prevDir := dir
		dir = filepath.Dir(dir)
		// Is the parent directory the same as the child.
		if prevDir == dir {
			// If we are unable to determine a different parent from
			// the current directory (most likely we have hit the root '/').
			break
		}
	}
	// If no configs were found, return an error.
	if len(cfgs) == 0 {
		return nil, fmt.Errorf("no .gunkconfig found for %q", dir)
	}
	// Merge the found configs.
	// TODO(hhhapz): merge DocConfig and Format config.
	config := cfgs[0]
	for i := 1; i < len(cfgs); i++ {
		c := cfgs[i]
		// Set the protoc path + version to the first non-blank values found (if any).
		// They are visited in order of specificity, so a .gunkconfig in a child directory can
		// override the protoc configuration specified in its parent.
		if protocVer := c.ProtocVersion; config.ProtocVersion == "" {
			config.ProtocVersion = protocVer
		}
		if protocPath := c.ProtocPath; config.ProtocPath == "" {
			config.ProtocPath = protocPath
		}
		// Don't create duplicated generate single generators.
		for _, g := range config.Generators {
			if g.Single {
				continue
			}
			config.Generators = append(config.Generators, g)
		}
	}
	return config, nil
}

// from https://github.com/protocolbuffers/protobuf/blob/master/src/google/protobuf/compiler/main.cc
// hardcode what languages are built-in in protoc, rest must have their own generator binary
var ProtocBuiltinLanguages = map[string]bool{
	"cpp":    true,
	"java":   true,
	"python": true,
	"php":    true,
	"ruby":   true,
	"csharp": true,
	"objc":   true,
	"js":     true,
}

func LoadSingle(reader io.Reader, dir string) (*Config, error) {
	f, err := ini.Load(reader)
	if err != nil {
		return nil, fmt.Errorf("unable to parse ini file: %v", err)
	}
	config := &Config{
		Generators: make([]Generator, 0, len(f.AllSections())),
		DocsConfig: make(map[string]*DocConfig),
		Dir:        dir,
	}
	config.DocsConfig[DefaultTag] = &DocConfig{}
	for _, s := range f.AllSections() {
		var err error
		var gen *Generator
		name := s.Name()
		switch {
		case name == "":
			// This is the global section (unnamed section)
			err = handleGlobal(config, s)
		case name == "protoc":
			err = handleProtoc(config, s)
		case name == "generate":
			gen, err = handleGenerate(config, s, nil)
		case name == "format":
			err = handleFormat(config, s)
		case strings.HasPrefix(name, "generate "):
			// Check to see if we have the shorten version of a generate config:
			// [generate js].
			sParts := strings.Split(name, " ")
			if len(sParts) != 2 {
				return nil, fmt.Errorf("generate section name should have 2 values, not %d", len(sParts))
			}
			gen, err = handleGenerate(config, s, &sParts[1])
		case strings.HasPrefix(name, "doc "):
			sParts := strings.Split(name, " ")
			if len(sParts) != 2 {
				return nil, fmt.Errorf("doc section name should have 2 values, not %d", len(sParts))
			}
			err = handleDoc(config, s, sParts[1])
		default:
			return nil, fmt.Errorf("unknown section %q", s.Name())
		}
		if err != nil {
			return nil, err
		}
		if gen != nil {
			config.Generators = append(config.Generators, *gen)
		}
	}
	return config, nil
}

func handleProtoc(config *Config, section *parser.Section) error {
	for _, k := range section.RawKeys() {
		v := strings.TrimSpace(section.GetRaw(k))
		switch k {
		case "path":
			config.ProtocPath = v
		case "version":
			config.ProtocVersion = v
		default:
			return fmt.Errorf("unexpected key %q in protoc section", k)
		}
	}
	return nil
}

func handleGenerate(config *Config, section *parser.Section, shorthand *string) (*Generator, error) {
	keys := section.RawKeys()
	gen := &Generator{
		Params:    make([]KeyValue, 0, len(keys)),
		ConfigDir: config.Dir,
	}

	if shorthand != nil {
		generator := strings.Trim(*shorthand, "\"")
		// Is this shortened generator a protoc-gen-* binary, or
		// should it be passed to protoc.
		// We ignore the binary path since we don't do the same for the
		// normal generate section. If we start using the binary path here
		// we should also use it for the normal generate section.
		switch {
		case generator == "doc":
			gen.Command = generator
		case ProtocBuiltinLanguages[generator]:
			gen.ProtocGen = generator
		default:
			gen.Command = "protoc-gen-" + generator
		}
		gen.Shortened = true // for vetting
	}

	for _, k := range keys {
		v := strings.TrimSpace(section.GetRaw(k))
		switch k {
		case "command":
			if shorthand != nil {
				return nil, fmt.Errorf("'command' or 'protoc' may not be specified in generate shorthand")
			}
			if gen.ProtocGen != "" {
				return nil, fmt.Errorf("only one 'command' or 'protoc' allowed")
			}
			gen.Command = v
		case "protoc":
			if shorthand != nil {
				return nil, fmt.Errorf("'command' or 'protoc' may not be specified in generate shorthand")
			}
			if gen.Command != "" {
				return nil, fmt.Errorf("only one 'command' or 'protoc' allowed")
			}
			gen.ProtocGen = v
		case "plugin_version":
			gen.PluginVersion = v
		case "out":
			gen.Out = v
		case "fix_paths_postproc":
			p, err := strconv.ParseBool(v)
			if err != nil {
				return nil, fmt.Errorf("cannot parse fix_paths: %w", err)
			}
			gen.FixPaths = p
		case "json_tag_postproc":
			p, err := strconv.ParseBool(v)
			if err != nil {
				return nil, fmt.Errorf("cannot parse json_tag_postproc: %w", err)
			}
			gen.JSONPostProc = p
		case "generate_single":
			single, err := strconv.ParseBool(v)
			if err != nil {
				return nil, fmt.Errorf("cannot parse generate_single: %w", err)
			}
			gen.Single = single

		default:
			v = replacePATH(v, config.Dir)
			gen.Params = append(gen.Params, KeyValue{k, v})
		}
	}

	if gen.Command == "" && gen.ProtocGen == "" {
		return nil, fmt.Errorf("either 'command' or 'protoc' must be specified")
	}

	// Validate language-specific options now that we are done as we should
	// have figured out language by now.
	lang := gen.Code()
	if gen.FixPaths && lang != "js" && lang != "ts" {
		return nil, fmt.Errorf("fix_paths_postproc can only be set for js and ts. Enabled on %q", lang)
	}
	if gen.JSONPostProc && lang != "go" {
		return nil, fmt.Errorf("json_tag_postproc can only be set for go. Enabled on %q", lang)
	}

	return gen, nil
}

func handleDoc(config *Config, section *parser.Section, tag string) error {
	docConfig := &DocConfig{}
	for _, k := range section.RawKeys() {
		switch k {
		case "name":
			docConfig.Name = section.Get(k)
		case "preamble":
			docConfig.Preamble = section.Get(k)
		case "packages":
			paths := strings.Split(section.Get(k), ",")
			for _, p := range paths {
				p = strings.TrimSpace(p)
				if p == "" {
					return fmt.Errorf("empty path in packages")
				}
				docConfig.Packages = append(docConfig.Packages, p)
			}
		case "weight":
			w, err := strconv.Atoi(section.Get(k))
			if err != nil {
				return fmt.Errorf("cannot parse weight: %w", err)
			}
			docConfig.Weight = int(w)
		default:
			return fmt.Errorf("unknown key %q in doc section", k)
		}
	}
	// add to docs config
	config.DocsConfig[tag] = docConfig
	return nil
}

func handleGlobal(config *Config, section *parser.Section) error {
	for _, k := range section.RawKeys() {
		v := strings.TrimSpace(section.GetRaw(k))
		switch k {
		case "out":
			config.Out = v
		case "import_path":
			config.ImportPath = v
		default:
			return fmt.Errorf("unexpected key %q in global section", k)
		}
	}
	return nil
}

func handleFormat(config *Config, section *parser.Section) error {
	for _, k := range section.RawKeys() {
		v := strings.TrimSpace(section.GetRaw(k))
		switch k {
		case "snake_case_json":
			enableJSON, err := strconv.ParseBool(v)
			if err != nil {
				return err
			}
			config.Format.JSON = enableJSON
		case "initialisms":
			if v == "" {
				return fmt.Errorf("initialisms must be a comma-separated list of initialisms to add")
			}
			config.Format.Initialisms = strings.Split(v, ",")
		case "reorder_pb":
			reorder, err := strconv.ParseBool(v)
			if err != nil {
				return err
			}
			config.Format.PB = reorder
		default:
			return fmt.Errorf("unexpected key %q in format section", k)
		}
	}
	return nil
}

// replacePATH processes the provided value, and replaces $PATH with the config
// directory.
func replacePATH(value string, path string) string {
	return strings.ReplaceAll(value, "$PATH", path)
}
