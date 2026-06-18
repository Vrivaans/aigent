package mcpcatalog

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	TransportStdio  = "stdio"
	TransportStream = "stream"
)

var (
	idPattern      = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
	aliasPattern   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)
	versionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.]+)?$`)
	placeholderRE  = regexp.MustCompile(`\{\{(\w+)\}\}`)
)

// Manifest describes an installable MCP template from the catalog.
type Manifest struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Version       string            `json:"version"`
	Transport     string            `json:"transport"`
	DefaultAlias  string            `json:"default_alias"`
	Tags          []string          `json:"tags"`
	ParamDefaults map[string]string `json:"param_defaults"`
	Stdio         *StdioSpec        `json:"stdio,omitempty"`
	Stream        *StreamSpec       `json:"stream,omitempty"`
}

type StdioSpec struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

type StreamSpec struct {
	BaseURL              string            `json:"base_url"`
	Headers              map[string]string `json:"headers"`
	DisableStandaloneSSE bool              `json:"disable_standalone_sse"`
}

// ParseManifest decodes and validates JSON manifest content.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if err := ValidateManifest(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// ValidateManifest checks manifest fields and transport-specific requirements.
func ValidateManifest(m *Manifest) error {
	if m == nil {
		return fmt.Errorf("manifest is nil")
	}
	m.ID = strings.TrimSpace(m.ID)
	m.Name = strings.TrimSpace(m.Name)
	m.Version = strings.TrimSpace(m.Version)
	m.Transport = strings.TrimSpace(m.Transport)
	m.DefaultAlias = strings.TrimSpace(m.DefaultAlias)

	if m.ID == "" {
		return fmt.Errorf("id is required")
	}
	if !idPattern.MatchString(m.ID) {
		return fmt.Errorf("id %q has invalid format", m.ID)
	}
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(m.Name) > 128 {
		return fmt.Errorf("name exceeds 128 characters")
	}
	if m.Version == "" {
		return fmt.Errorf("version is required")
	}
	if !versionPattern.MatchString(m.Version) {
		return fmt.Errorf("version %q must be semver (e.g. 1.0.0)", m.Version)
	}
	if m.DefaultAlias == "" {
		m.DefaultAlias = m.ID
	}
	if !aliasPattern.MatchString(m.DefaultAlias) {
		return fmt.Errorf("default_alias %q has invalid format", m.DefaultAlias)
	}

	switch m.Transport {
	case TransportStdio:
		if m.Stdio == nil {
			return fmt.Errorf("stdio block is required for transport %q", m.Transport)
		}
		if m.Stream != nil {
			return fmt.Errorf("stream block must be omitted for transport %q", m.Transport)
		}
		return validateStdio(m.Stdio)
	case TransportStream:
		if m.Stream == nil {
			return fmt.Errorf("stream block is required for transport %q", m.Transport)
		}
		if m.Stdio != nil {
			return fmt.Errorf("stdio block must be omitted for transport %q", m.Transport)
		}
		return validateStream(m.Stream)
	default:
		return fmt.Errorf("transport %q must be %q or %q", m.Transport, TransportStdio, TransportStream)
	}
}

func validateStdio(s *StdioSpec) error {
	s.Command = strings.TrimSpace(s.Command)
	if s.Command == "" {
		return fmt.Errorf("stdio.command is required")
	}
	if len(s.Command) > 512 {
		return fmt.Errorf("stdio.command exceeds 512 characters")
	}
	if len(s.Args) > 64 {
		return fmt.Errorf("stdio.args exceeds 64 entries")
	}
	if s.Env == nil {
		s.Env = map[string]string{}
	}
	return nil
}

func validateStream(s *StreamSpec) error {
	s.BaseURL = strings.TrimSpace(s.BaseURL)
	if s.BaseURL == "" {
		return fmt.Errorf("stream.base_url is required")
	}
	if len(s.BaseURL) > 2048 {
		return fmt.Errorf("stream.base_url exceeds 2048 characters")
	}
	if s.Headers == nil {
		s.Headers = map[string]string{}
	}
	return nil
}

// ResolveAlias returns the alias for install (request override or manifest default).
func (m *Manifest) ResolveAlias(requestAlias string) string {
	if a := strings.TrimSpace(requestAlias); a != "" {
		return a
	}
	return m.DefaultAlias
}

// ResolvedStdioConfig applies param substitution to stdio command/args/env.
func (m *Manifest) ResolvedStdioConfig(params map[string]string) (command string, args []string, env map[string]string, err error) {
	if m.Stdio == nil {
		return "", nil, nil, fmt.Errorf("manifest has no stdio block")
	}
	merged := mergeParams(m.ParamDefaults, params)
	args, err = applyPlaceholders(m.Stdio.Args, merged)
	if err != nil {
		return "", nil, nil, err
	}
	env = map[string]string{}
	for k, v := range m.Stdio.Env {
		resolved, err := applyPlaceholders([]string{v}, merged)
		if err != nil {
			return "", nil, nil, fmt.Errorf("stdio.env[%s]: %w", k, err)
		}
		env[k] = resolved[0]
	}
	return m.Stdio.Command, args, env, nil
}

// ResolvedStreamConfig returns stream settings with optional header overrides from params (keys prefixed with header_).
func (m *Manifest) ResolvedStreamConfig(params map[string]string) (baseURL string, headers map[string]string, disableSSE bool, err error) {
	if m.Stream == nil {
		return "", nil, false, fmt.Errorf("manifest has no stream block")
	}
	headers = map[string]string{}
	for k, v := range m.Stream.Headers {
		headers[k] = v
	}
	for k, v := range params {
		if strings.HasPrefix(k, "header_") {
			headers[strings.TrimPrefix(k, "header_")] = v
		}
	}
	return m.Stream.BaseURL, headers, m.Stream.DisableStandaloneSSE, nil
}

// RequiredParamKeys lists {{placeholder}} names used in stdio args/env.
func (m *Manifest) RequiredParamKeys() []string {
	if m == nil || m.Stdio == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var keys []string
	collect := func(values ...string) {
		for _, raw := range values {
			for _, match := range placeholderRE.FindAllStringSubmatch(raw, -1) {
				key := match[1]
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				keys = append(keys, key)
			}
		}
	}
	collect(m.Stdio.Args...)
	for _, v := range m.Stdio.Env {
		collect(v)
	}
	return keys
}

// PublicView is the catalog entry returned by the list API.
type PublicView struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Version        string            `json:"version"`
	Transport      string            `json:"transport"`
	DefaultAlias   string            `json:"default_alias"`
	Tags           []string          `json:"tags"`
	ParamDefaults  map[string]string `json:"param_defaults"`
	RequiredParams []string          `json:"required_params"`
}

// PublicView returns a safe summary for catalog listing.
func (m *Manifest) PublicView() PublicView {
	if m == nil {
		return PublicView{}
	}
	defaults := map[string]string{}
	for k, v := range m.ParamDefaults {
		defaults[k] = v
	}
	tags := append([]string(nil), m.Tags...)
	return PublicView{
		ID:             m.ID,
		Name:           m.Name,
		Description:    m.Description,
		Version:        m.Version,
		Transport:      m.Transport,
		DefaultAlias:   m.DefaultAlias,
		Tags:           tags,
		ParamDefaults:  defaults,
		RequiredParams: m.RequiredParamKeys(),
	}
}

func mergeParams(defaults, overrides map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range defaults {
		out[k] = v
	}
	for k, v := range overrides {
		out[k] = v
	}
	return out
}

func applyPlaceholders(values []string, params map[string]string) ([]string, error) {
	out := make([]string, len(values))
	for i, raw := range values {
		missing := placeholderRE.FindAllStringSubmatch(raw, -1)
		resolved := raw
		for _, m := range missing {
			key := m[1]
			val, ok := params[key]
			if !ok || val == "" {
				return nil, fmt.Errorf("missing param %q for placeholder {{%s}}", key, key)
			}
			resolved = strings.ReplaceAll(resolved, "{{"+key+"}}", val)
		}
		if placeholderRE.MatchString(resolved) {
			return nil, fmt.Errorf("unresolved placeholders in %q", raw)
		}
		out[i] = resolved
	}
	return out, nil
}
