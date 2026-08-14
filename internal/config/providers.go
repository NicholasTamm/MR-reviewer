package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
)

type WireAPI string

const (
	WireOpenAI    WireAPI = "openai"
	WireAnthropic WireAPI = "anthropic"
)

var BuiltinProviderNames = map[string]struct{}{
	"anthropic": {},
	"openai":    {},
	"xai":       {},
	"google":    {},
	"gemini":    {},
	"kimi":      {},
	"deepseek":  {},
	"echo":      {},
	"ollama":    {},
}

var providerNameRE = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

type CustomProvider struct {
	Name      string
	BaseURL   string
	API       WireAPI
	APIKeyEnv string
	Models    []string
}

type ProviderEndpoint struct {
	BaseURL   string
	APIKeyEnv string
}

func (e ProviderEndpoint) Active() bool {
	return strings.TrimSpace(e.BaseURL) != "" || strings.TrimSpace(e.APIKeyEnv) != ""
}

type ProvidersFile struct {
	Customs   []CustomProvider
	Endpoints map[string]ProviderEndpoint
}

type providersFileEntry struct {
	NPM       string             `json:"npm,omitempty"`
	API       string             `json:"api,omitempty"`
	Models    []string           `json:"models,omitempty"`
	Options   *providersFileOpts `json:"options,omitempty"`
	BaseURL   string             `json:"baseURL,omitempty"`
	APIKeyEnv string             `json:"apiKeyEnv,omitempty"`
}

type providersFileOpts struct {
	BaseURL string `json:"baseURL,omitempty"`
	APIKey  string `json:"apiKey,omitempty"`
}

func LoadProviders() ProvidersFile {
	pf, err := ReadProvidersFile(ProvidersPath())
	if err != nil {
		return ProvidersFile{}
	}
	return pf
}

func ReadProvidersFile(path string) (ProvidersFile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ProvidersFile{}, nil
	}
	if err != nil {
		return ProvidersFile{}, err
	}
	return ParseProvidersFile(data)
}

func ParseProvidersFile(data []byte) (ProvidersFile, error) {
	stripped, err := stripJSONC(data)
	if err != nil {
		return ProvidersFile{}, err
	}
	stripped = bytesTrimSpace(stripped)
	if len(stripped) == 0 {
		return ProvidersFile{}, nil
	}
	if stripped[0] != '{' {
		return ProvidersFile{}, fmt.Errorf("providers file must be a JSON object")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(stripped, &raw); err != nil {
		return ProvidersFile{}, err
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out ProvidersFile
	for _, key := range keys {
		k := strings.ToLower(strings.TrimSpace(key))
		if strings.HasPrefix(k, "disable-default") {
			continue
		}
		var entry providersFileEntry
		if err := json.Unmarshal(raw[key], &entry); err != nil {
			return ProvidersFile{}, fmt.Errorf("provider %q: %w", key, err)
		}
		id := CanonicalProviderID(key)
		if id == "" {
			return ProvidersFile{}, errors.New("empty provider id")
		}
		if _, builtin := BuiltinProviderNames[id]; builtin {
			if id == "echo" {
				continue
			}
			if ep := entry.toEndpoint(); ep.Active() {
				if out.Endpoints == nil {
					out.Endpoints = map[string]ProviderEndpoint{}
				}
				out.Endpoints[id] = ep
			}
			continue
		}
		p, err := entry.toCustom(key)
		if err != nil {
			return ProvidersFile{}, fmt.Errorf("provider %q: %w", key, err)
		}
		out.Customs = append(out.Customs, p)
	}
	return out, nil
}

func CanonicalProviderID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "gemini" {
		return "google"
	}
	return id
}

func FindCustom(list []CustomProvider, name string) (CustomProvider, bool) {
	name = CanonicalProviderID(name)
	for _, p := range list {
		if p.Name == name {
			return p, true
		}
	}
	return CustomProvider{}, false
}

func (e providersFileEntry) toEndpoint() ProviderEndpoint {
	baseURL := strings.TrimSpace(e.BaseURL)
	apiKeyRaw := ""
	if e.Options != nil {
		if u := strings.TrimSpace(e.Options.BaseURL); u != "" {
			baseURL = u
		}
		apiKeyRaw = strings.TrimSpace(e.Options.APIKey)
	}
	apiKeyEnv := NormalizeAPIKeyEnv(e.APIKeyEnv)
	if apiKeyEnv == "" && apiKeyRaw != "" {
		if name, ok := EnvRefName(apiKeyRaw); ok {
			apiKeyEnv = name
		}
	}
	if ContainsEnvRef(baseURL) {
		baseURL = strings.TrimRight(strings.TrimSpace(ExpandEnv(baseURL)), "/")
	} else {
		baseURL = strings.TrimRight(baseURL, "/")
	}
	return ProviderEndpoint{BaseURL: baseURL, APIKeyEnv: apiKeyEnv}
}

func (e providersFileEntry) toCustom(key string) (CustomProvider, error) {
	id := CanonicalProviderID(key)
	if !providerNameRE.MatchString(id) {
		return CustomProvider{}, fmt.Errorf("provider name %q must be a lowercase slug", key)
	}
	ep := e.toEndpoint()
	if err := validateBaseURL(ep.BaseURL); err != nil {
		return CustomProvider{}, err
	}
	api := WireAPI(strings.ToLower(strings.TrimSpace(e.API)))
	if api == "" {
		api = wireFromNPM(e.NPM)
	}
	if api != WireOpenAI && api != WireAnthropic {
		if api == "" {
			api = WireOpenAI
		} else if api != WireOpenAI && api != WireAnthropic {
			return CustomProvider{}, fmt.Errorf("unknown api %q (want openai or anthropic)", api)
		}
	}
	return CustomProvider{
		Name:      id,
		BaseURL:   ep.BaseURL,
		API:       api,
		APIKeyEnv: ep.APIKeyEnv,
		Models:    e.Models,
	}, nil
}

func wireFromNPM(npm string) WireAPI {
	n := strings.ToLower(npm)
	if strings.Contains(n, "anthropic") {
		return WireAnthropic
	}
	return WireOpenAI
}

func validateBaseURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("baseURL is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("baseURL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("baseURL must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("baseURL must include a host")
	}
	return nil
}

func bytesTrimSpace(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && (b[i] == ' ' || b[i] == '\n' || b[i] == '\r' || b[i] == '\t') {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\n' || b[j-1] == '\r' || b[j-1] == '\t') {
		j--
	}
	return b[i:j]
}
