// Package config loads reasonix-mini TOML configuration.
//
// Resolution order follows the Reasonix SPEC §5: project ./reasonix-mini.toml
// > the user config file > built-in defaults. Later sources are layered onto
// earlier ones: a project file that declares providers replaces the user
// providers entirely, while scalar fields fall back through the chain.
// API keys are never stored in config files; providers name a secret via
// api_key_env and the value is read from the environment at startup.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ProviderConfig is one vendor endpoint (SPEC §3.1). An OpenAI-compatible
// vendor is a config instance of kind "openai", not new code.
type ProviderConfig struct {
	Name          string   `toml:"name"`
	Kind          string   `toml:"kind"`
	BaseURL       string   `toml:"base_url"`
	Model         string   `toml:"model"`
	Models        []string `toml:"models"`
	Default       string   `toml:"default"`
	APIKeyEnv     string   `toml:"api_key_env"`
	ContextWindow int      `toml:"context_window"`
}

// AgentConfig holds the cache-stable system prompt and loop parameters.
type AgentConfig struct {
	SystemPrompt     string  `toml:"system_prompt"`
	SystemPromptFile string  `toml:"system_prompt_file"`
	Temperature      float64 `toml:"temperature"`
	MaxSteps         int     `toml:"max_steps"`
}

// ToolsConfig filters the compile-time builtin set.
type ToolsConfig struct {
	Enabled []string `toml:"enabled"`
}

// PermissionsConfig is the static half of the permission policy (SPEC §3.7).
type PermissionsConfig struct {
	Mode  string   `toml:"mode"` // ask|allow|deny
	Allow []string `toml:"allow"`
	Ask   []string `toml:"ask"`
	Deny  []string `toml:"deny"`
}

// Config is the assembled runtime configuration.
type Config struct {
	DefaultModel string            `toml:"default_model"`
	Providers    []ProviderConfig  `toml:"providers"`
	Agent        AgentConfig       `toml:"agent"`
	Tools        ToolsConfig       `toml:"tools"`
	Permissions  PermissionsConfig `toml:"permissions"`

	projectMemory string // REASONIX.md / AGENTS.md content, appended to the stable prefix
}

// UserConfigPath returns the user config file location (~/.reasonix-mini),
// overridable via REASONIX_MINI_HOME for tests.
func UserConfigPath() string {
	if home := os.Getenv("REASONIX_MINI_HOME"); home != "" {
		return filepath.Join(home, "config.toml")
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".reasonix-mini", "config.toml")
	}
	return filepath.Join(".reasonix-mini-home", "config.toml") // last resort: cwd-relative
}

// ProjectConfigPath is the workspace-local config file.
func ProjectConfigPath() string { return "reasonix-mini.toml" }

// Load reads config sources in precedence order and applies defaults.
func Load() (*Config, error) {
	cfg := &Config{}
	// Low precedence first: user file, then project file layered on top.
	for _, p := range []string{UserConfigPath(), ProjectConfigPath()} {
		if _, err := os.Stat(p); err != nil {
			continue // missing source is fine; it just contributes nothing
		}
		var src Config
		if _, err := toml.DecodeFile(p, &src); err != nil {
			return nil, fmt.Errorf("config: decode %s: %w", p, err)
		}
		merge(cfg, &src)
	}
	cfg.applyDefaults()
	cfg.loadProjectMemory()
	return cfg, nil
}

// merge layers a higher-precedence source onto the accumulated config.
func merge(dst, src *Config) {
	if src.DefaultModel != "" {
		dst.DefaultModel = src.DefaultModel
	}
	if len(src.Providers) > 0 {
		dst.Providers = src.Providers // lists replace, not append
	}
	if src.Agent.SystemPrompt != "" {
		dst.Agent.SystemPrompt = src.Agent.SystemPrompt
	}
	if src.Agent.SystemPromptFile != "" {
		dst.Agent.SystemPromptFile = src.Agent.SystemPromptFile
	}
	if src.Agent.Temperature != 0 {
		dst.Agent.Temperature = src.Agent.Temperature
	}
	if src.Agent.MaxSteps != 0 {
		dst.Agent.MaxSteps = src.Agent.MaxSteps
	}
	if len(src.Tools.Enabled) > 0 {
		dst.Tools.Enabled = src.Tools.Enabled
	}
	if src.Permissions.Mode != "" {
		dst.Permissions.Mode = src.Permissions.Mode
	}
	if len(src.Permissions.Allow) > 0 {
		dst.Permissions.Allow = src.Permissions.Allow
	}
	if len(src.Permissions.Ask) > 0 {
		dst.Permissions.Ask = src.Permissions.Ask
	}
	if len(src.Permissions.Deny) > 0 {
		dst.Permissions.Deny = src.Permissions.Deny
	}
}

func (c *Config) applyDefaults() {
	if c.Agent.MaxSteps <= 0 {
		c.Agent.MaxSteps = 50
	}
	if c.Permissions.Mode == "" {
		c.Permissions.Mode = "ask"
	}
	if len(c.Providers) == 0 {
		c.Providers = []ProviderConfig{{
			Name: "deepseek", Kind: "openai",
			BaseURL: "https://api.deepseek.com",
			Model:   "deepseek-chat", APIKeyEnv: "DEEPSEEK_API_KEY",
		}}
	}
}

// loadProjectMemory folds standing project instructions into the config.
// These files are the analog of the parent repo's REASONIX.md convention and
// join the cache-stable system prefix, so they are read once at startup.
func (c *Config) loadProjectMemory() {
	for _, name := range []string{"REASONIX.md", "AGENTS.md"} {
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		c.projectMemory += fmt.Sprintf("\n\n# Project instructions (%s)\n\n%s", name, data)
	}
}

// SystemPrompt returns the full cache-stable prefix: base prompt + project
// memory. It is built once per process and never mutated mid-session (SPEC:
// cache-first — the prefix must stay byte-stable across turns).
func (c *Config) SystemPrompt() string {
	base := c.Agent.SystemPrompt
	if base == "" && c.Agent.SystemPromptFile != "" {
		if data, err := os.ReadFile(c.Agent.SystemPromptFile); err == nil {
			base = string(data)
		}
	}
	if base == "" {
		base = "You are reasonix-mini, a coding agent running in a terminal. " +
			"Use the tools to read and modify files and to run commands. " +
			"Prefer small, verifiable steps; report what you verified."
	}
	return base + c.projectMemory
}

// ResolveModel maps a model reference to a provider and concrete model name.
// Accepted forms (SPEC §3.1): provider name → its default model, a bare model
// name → the provider that declares it, or "provider/model".
func (c *Config) ResolveModel(ref string) (*ProviderConfig, string, error) {
	if ref == "" && c.DefaultModel != "" {
		ref = c.DefaultModel
	}
	var fallback *ProviderConfig
	for i := range c.Providers {
		p := &c.Providers[i]
		if ref == "" || ref == p.Name {
			if name := p.defaultModelName(); name != "" {
				return p, name, nil
			}
		}
		if ref != "" {
			if p.Name+"/"+p.defaultModelName() == ref {
				return p, p.defaultModelName(), nil
			}
			if i == 0 {
				fallback = p
			}
		}
	}
	if ref != "" {
		for i := range c.Providers {
			p := &c.Providers[i]
			if p.Model == ref {
				return p, ref, nil
			}
			for _, m := range p.Models {
				if m == ref {
					return p, ref, nil
				}
			}
		}
	}
	if fallback != nil {
		name := fallback.defaultModelName()
		if name == "" {
			return nil, "", fmt.Errorf("config: provider %q declares no model", fallback.Name)
		}
		return fallback, name, nil
	}
	return nil, "", fmt.Errorf("config: cannot resolve model reference %q and no providers configured", ref)
}

func (p *ProviderConfig) defaultModelName() string {
	if p.Default != "" {
		return p.Default
	}
	if p.Model != "" {
		return p.Model
	}
	if len(p.Models) > 0 {
		return p.Models[0]
	}
	return ""
}

// APIKey resolves the provider secret from the environment named by api_key_env.
func (p *ProviderConfig) APIKey() string {
	if p.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(p.APIKeyEnv)
}

// Sample returns a ready-to-edit starter config, used by the setup command.
func Sample() string {
	return `# reasonix-mini configuration. Resolution: project ./reasonix-mini.toml >
# ~/.reasonix-mini/config.toml > built-in defaults. Secrets live in the
# environment, named by api_key_env — never in this file.
default_model = "deepseek"

[[providers]]
name        = "deepseek"
kind        = "openai"
base_url    = "https://api.deepseek.com"
model       = "deepseek-chat"
api_key_env = "DEEPSEEK_API_KEY"

[agent]
# system_prompt = "..."   # or system_prompt_file = "prompts/agent.md"
temperature = 0.0
max_steps   = 50

[tools]
enabled = []   # empty = all builtins

[permissions]
mode  = "ask"                                   # writer fallback: ask|allow|deny
deny  = ["Bash(rm -rf*)"]                       # hard-blocked in every mode
allow = ["Bash(go test:*)", "Bash(git status:*)"]
ask   = []
`
}
