package config

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/knq/ini"
	"github.com/knq/ini/parser"
)

const (
	goModFilename = "go.mod"
	gitFilename   = ".git"
)

type KeyValue struct {
	Key   string
	Value string
}

type Generator struct {
	ProtocGen string // The type of protoc generator that should be run; js, python, etc.
	Command   string
	Params    []KeyValue
	ConfigDir string
	Out       string
}

func (g Generator) IsProtoc() bool {
	return g.ProtocGen != ""
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

// ParamStringWithOut will return the generator paramaters formatted
// for protoc, including where protoc should output the generated files.
// It will use 'packageDir' if no 'out' key was set in the config.
func (g Generator) ParamStringWithOut(packageDir string) string {
	// If no out path was specified, use the package directory.
	outPath := g.OutPath(packageDir)
	params := g.ParamString()
	if params == "" {
		return outPath
	}
	return params + ":" + outPath
}

// OutPath determines the path for a generator to write generated files to. It
// will use 'packageDir' if no 'out' key was set in the config.
func (g Generator) OutPath(packageDir string) string {
	if g.Out == "" {
		return packageDir
	}
	if filepath.IsAbs(g.Out) {
		return g.Out
	}
	return filepath.Join(g.ConfigDir, g.Out)
}

type Config struct {
	Dir        string
	Out        string
	Generators []Generator
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
			cfg, err := load(reader)
			if err != nil {
				return nil, fmt.Errorf("error loading %q: %v", configPath, err)
			}

			cfg.Dir = dir
			// Patch in the directory of where to output the generated
			// files. And patch in the 'out' path if it has been set globally,
			// and not in the generate section.
			for i, gen := range cfg.Generators {
				cfg.Generators[i].ConfigDir = dir
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
		return nil, fmt.Errorf("no .gunkconfig found")
	}

	// Merge the found configs.
	cfg := cfgs[0]
	for i := 1; i < len(cfgs); i++ {
		cfg.Generators = append(cfg.Generators, cfgs[i].Generators...)
	}

	return cfg, nil
}

func load(reader io.Reader) (*Config, error) {
	f, err := ini.Load(reader)
	if err != nil {
		return nil, fmt.Errorf("unable to parse ini file: %v", err)
	}
	config := &Config{
		Generators: make([]Generator, 0, len(f.AllSections())),
	}
	for _, s := range f.AllSections() {
		var err error
		var gen *Generator
		name := s.Name()
		switch {
		case name == "":
			// This is the global section (unnamed section)
			if err := handleGlobal(config, s); err != nil {
				return nil, err
			}
			continue
		case name == "generate":
			gen, err = handleGenerate(s)
		case strings.HasPrefix(name, "generate"):
			// Check to see if we have the shorten version of a generate config:
			// [generate js].
			sParts := strings.Split(name, " ")
			if len(sParts) != 2 {
				return nil, fmt.Errorf("generate section name should have 2 values, not %d", len(sParts))
			}

			gen, err = handleGenerate(s)
			generator := strings.Trim(sParts[1], "\"")
			// Is this shortened generator a protoc-gen-* binary, or
			// should it be passed to protoc.
			// We ignore the binary path since we don't do the same for the
			// normal generate section. If we start using the binary path here
			// we should also use it for the normal generate section.
			if _, err := exec.LookPath("protoc-gen-" + generator); err == nil {
				gen.Command = "protoc-gen-" + generator
			} else {
				gen.ProtocGen = generator
			}
		default:
			return nil, fmt.Errorf("unknown section %q", s.Name())
		}
		if err != nil {
			return nil, err
		}
		config.Generators = append(config.Generators, *gen)
	}
	return config, nil
}

func handleGenerate(section *parser.Section) (*Generator, error) {
	keys := section.RawKeys()
	gen := &Generator{
		Params: make([]KeyValue, 0, len(keys)),
	}
	for _, k := range keys {
		v := section.GetRaw(k)
		switch k {
		case "command":
			if gen.ProtocGen != "" {
				return nil, fmt.Errorf("only one 'command' or 'protoc' allowed")
			}
			gen.Command = v
		case "protoc":
			if gen.Command != "" {
				return nil, fmt.Errorf("only one 'command' or 'protoc' allowed")
			}
			gen.ProtocGen = v
		case "out":
			gen.Out = v
		default:
			gen.Params = append(gen.Params, KeyValue{k, v})
		}
	}
	return gen, nil
}

func handleGlobal(config *Config, section *parser.Section) error {
	for _, k := range section.RawKeys() {
		v := section.GetRaw(k)
		switch k {
		case "out":
			config.Out = v
		default:
			return fmt.Errorf("unexpected key %q in global section", k)
		}
	}
	return nil
}
