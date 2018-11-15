package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/knq/ini"
	"github.com/knq/ini/parser"
)

type KeyValue struct {
	Key   string
	Value string
}

type Generator struct {
	ProtocGen string // The type of protoc generator that should be run; js, python, etc.
	Command   string
	params    []KeyValue
	Out       string
}

func (g Generator) IsProtoc() bool {
	return g.ProtocGen != ""
}

func (g Generator) Params() string {
	params := make([]string, len(g.params))
	for i, p := range g.params {
		if p.Value != "" {
			params[i] = fmt.Sprintf("%s=%s", p.Key, p.Value)
		} else {
			params[i] = fmt.Sprintf("%s", p.Key)
		}
	}
	return strings.Join(params, ",")
}

func (g Generator) ParamsWithOut() string {
	params := g.Params()
	if params == "" {
		return g.Out
	}
	return params + ":" + g.Out
}

type Config struct {
	Dir        string
	Generators []Generator
}

// Load will attempt to find the .gunkconfig in the 'dir', working
// its way up to each parent looking for a .gunkconfig. Currently,
// Load will only stop when it is unable to go any further up the
// directory structure.
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
	startDir := dir
	var cfg *Config
	for {
		configPath := filepath.Join(dir, ".gunkconfig")
		reader, err := os.Open(configPath)
		if err != nil {
			prevDir := dir
			dir = filepath.Dir(dir)
			// If we are unable to go any further up the directory
			// structure.
			if prevDir == dir {
				return nil, fmt.Errorf("could not find a .gunkconfig")
			}
			continue
		}
		defer reader.Close()
		cfg, err = load(reader)
		if err != nil {
			return nil, fmt.Errorf("error loading %q: %v", configPath, err)
		}
		cfg.Dir = startDir
		// Patch in the directory of where to output the generated
		// files.
		// TODO(vishen): Make this configurable
		for i := range cfg.Generators {
			cfg.Generators[i].Out = startDir
		}
		break
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
			// TODO: Sometimes the ini parser returns an empty first section name.
			continue
		case name == "generate":
			gen, err = handleGenerate(s)
		case strings.HasPrefix(name, "generate"):
			sParts := strings.Split(name, " ")
			if len(sParts) != 2 {
				return nil, fmt.Errorf("generate section name should have 2 values, not %d", len(sParts))
			}
			// Check to see if we have the shorten version of a generate config:
			// [generate js].
			gen, err = handleGenerate(s)
			gen.ProtocGen = strings.Trim(sParts[1], "\"")
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
		params: make([]KeyValue, 0, len(keys)),
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
		default:
			gen.params = append(gen.params, KeyValue{k, v})
		}
	}
	return gen, nil
}
