package config

// This file implements key-path access to the configuration: dotted paths
// like "project.name" or "agents.claude-code/opus.model", resolved against
// the Config schema via yaml struct tags. Reads operate on the effective
// (defaults-applied) config; writes edit the raw .metis/project.yaml through the
// yaml.v3 node tree so comments and unrelated formatting survive.

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/techspeque/metis/internal/fsutil"
)

// Lookup returns the value at a dotted key path in the config.
func Lookup(cfg *Config, path string) (any, error) {
	v := reflect.ValueOf(cfg).Elem()
	segs := strings.Split(path, ".")
	for i, seg := range segs {
		prefix := strings.Join(segs[:i], ".")
		switch v.Kind() {
		case reflect.Struct:
			f, ok := fieldByYAMLTag(v.Type(), seg)
			if !ok {
				return nil, unknownKeyError(v.Type(), prefix, seg)
			}
			v = v.FieldByIndex(f.Index)
		case reflect.Map:
			mv := v.MapIndex(reflect.ValueOf(seg))
			if !mv.IsValid() {
				return nil, fmt.Errorf("%q has no entry %q", prefix, seg)
			}
			v = mv
		default:
			return nil, fmt.Errorf("config key %q has no sub-key %q", prefix, seg)
		}
	}
	return v.Interface(), nil
}

// PathType resolves the schema type at a dotted key path, without needing a
// loaded config. Map segments accept any key (the entry need not exist yet).
func PathType(path string) (reflect.Type, error) {
	t := reflect.TypeOf(Config{})
	segs := strings.Split(path, ".")
	for i, seg := range segs {
		prefix := strings.Join(segs[:i], ".")
		switch t.Kind() {
		case reflect.Struct:
			f, ok := fieldByYAMLTag(t, seg)
			if !ok {
				return nil, unknownKeyError(t, prefix, seg)
			}
			t = f.Type
		case reflect.Map:
			t = t.Elem()
		default:
			return nil, fmt.Errorf("config key %q has no sub-key %q", prefix, seg)
		}
	}
	return t, nil
}

// fieldByYAMLTag finds a struct field by the name part of its yaml tag.
func fieldByYAMLTag(t reflect.Type, name string) (reflect.StructField, bool) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := strings.Split(f.Tag.Get("yaml"), ",")[0]
		if tag == name {
			return f, true
		}
	}
	return reflect.StructField{}, false
}

// unknownKeyError lists the valid keys at the failing level.
func unknownKeyError(t reflect.Type, prefix, seg string) error {
	var valid []string
	for i := 0; i < t.NumField(); i++ {
		if tag := strings.Split(t.Field(i).Tag.Get("yaml"), ",")[0]; tag != "" {
			valid = append(valid, tag)
		}
	}
	where := "top level"
	if prefix != "" {
		where = fmt.Sprintf("%q", prefix)
	}
	return fmt.Errorf("unknown config key %q at %s (valid: %s)", seg, where, strings.Join(valid, ", "))
}

// parseValueNode converts a raw CLI string into a yaml node matching the
// schema type at the target path.
func parseValueNode(t reflect.Type, raw string) (*yaml.Node, error) {
	scalar := func(tag, value string) *yaml.Node {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
	}

	switch t.Kind() {
	case reflect.String:
		return scalar("!!str", raw), nil
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("expected a boolean (true/false), got %q", raw)
		}
		return scalar("!!bool", strconv.FormatBool(b)), nil
	case reflect.Int:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("expected an integer, got %q", raw)
		}
		return scalar("!!int", strconv.Itoa(n)), nil
	case reflect.Slice:
		if t.Elem().Kind() != reflect.String {
			return nil, fmt.Errorf("this key is not settable from the CLI — edit .metis/project.yaml directly")
		}
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, item := range strings.Split(raw, ",") {
			seq.Content = append(seq.Content, scalar("!!str", strings.TrimSpace(item)))
		}
		return seq, nil
	default:
		return nil, fmt.Errorf("this key holds a %s and is not settable from the CLI — edit .metis/project.yaml directly", t.Kind())
	}
}

// SetInFile updates one key in a .metis/project.yaml file, preserving comments and
// unrelated formatting. Intermediate mappings are created as needed.
func SetInFile(filePath, keyPath, raw string) error {
	t, err := PathType(keyPath)
	if err != nil {
		return err
	}
	valNode, err := parseValueNode(t, raw)
	if err != nil {
		return fmt.Errorf("invalid value for %q: %w", keyPath, err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		doc = yaml.Node{
			Kind:    yaml.DocumentNode,
			Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}},
		}
	}

	mapping := doc.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return fmt.Errorf("config root is not a mapping")
	}

	segs := strings.Split(keyPath, ".")
	for _, seg := range segs[:len(segs)-1] {
		child := mappingValue(mapping, seg)
		if child == nil {
			child = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			mapping.Content = append(mapping.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: seg}, child)
		}
		if child.Kind != yaml.MappingNode {
			return fmt.Errorf("config key %q is not a section", seg)
		}
		mapping = child
	}

	last := segs[len(segs)-1]
	if existing := mappingValue(mapping, last); existing != nil {
		// Keep comments anchored to the old value.
		valNode.HeadComment = existing.HeadComment
		valNode.LineComment = existing.LineComment
		valNode.FootComment = existing.FootComment
		*existing = *valNode
	} else {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: last}, valNode)
	}

	// Confirm the result still parses into a valid Config shape.
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if _, err := Parse(buf.Bytes()); err != nil {
		return fmt.Errorf("refusing to write: result would not parse: %w", err)
	}

	return fsutil.WriteFileAtomic(filePath, buf.Bytes(), 0o644)
}

// mappingValue returns the value node for a key in a mapping, or nil.
func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}
