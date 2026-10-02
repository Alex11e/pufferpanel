package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/pufferpanel/pufferpanel/v3"
	"github.com/spf13/cast"
)

// pterodactylEggToTemplate converts an egg to a PufferPanel template. The install
// script is carried over as a bash step and must be reviewed before use, since it
// assumes Pterodactyl's separate installer container.
func pterodactylEggToTemplate(raw json.RawMessage) (*pufferpanel.Server, error) {
	var egg map[string]interface{}
	if err := json.Unmarshal(raw, &egg); err != nil {
		return nil, err
	}
	// Pterodactyl exports are PTDL_vN, Pelican exports PLCN_vN.
	if meta, ok := egg["meta"].(map[string]interface{}); ok {
		if version := cast.ToString(meta["version"]); version != "" && !strings.HasPrefix(version, "PTDL_") && !strings.HasPrefix(version, "PLCN_") {
			return nil, fmt.Errorf("unsupported egg format %q", version)
		}
	}
	startup := cast.ToString(egg["startup"])
	if startup == "" {
		// Pelican-style eggs list named startup commands instead.
		if commands, ok := egg["startup_commands"].(map[string]interface{}); ok {
			keys := make([]string, 0, len(commands))
			for key := range commands {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			if len(keys) > 0 {
				startup = cast.ToString(commands[keys[0]])
			}
		}
	}
	if startup == "" {
		return nil, fmt.Errorf("the egg does not contain a startup command")
	}
	name := slug(cast.ToString(egg["name"]))
	if name == "" {
		name = "imported-egg"
	}
	image := ""
	if images, ok := egg["docker_images"].(map[string]interface{}); ok {
		// Map iteration is random; pick the lexically first image for stable output.
		keys := make([]string, 0, len(images))
		for key := range images {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if image = cast.ToString(images[key]); image != "" {
				break
			}
		}
	}
	if image == "" {
		image = cast.ToString(egg["image"])
	}
	if image == "" {
		if images, ok := egg["images"].([]interface{}); ok && len(images) > 0 {
			image = cast.ToString(images[0])
		}
	}
	stop := cast.ToString(egg["stop"])
	if stop == "" {
		stop = pterodactylStop(egg["config"])
	}
	result := &pufferpanel.Server{
		Type: pufferpanel.Type{Type: "generic"}, Identifier: name, Display: cast.ToString(egg["name"]),
		Variables:             map[string]pufferpanel.Variable{},
		Execution:             pufferpanel.Execution{Command: pterodactylTokens(startup), StopCommand: stop, WorkingDirectory: ".", EnvironmentVariables: map[string]string{}},
		Environment:           pufferpanel.MetadataType{Type: "docker"},
		SupportedEnvironments: []pufferpanel.MetadataType{{Type: "docker"}},
	}
	if result.Display == "" {
		result.Display = name
	}
	// These are Pterodactyl's built-in allocation placeholders, not egg
	// variables. Map them to PufferPanel's conventional allocation variables.
	result.Variables["ip"] = pufferpanel.Variable{Type: pufferpanel.Type{Type: "string"}, Value: "0.0.0.0", Display: "IP", Required: true, UserEditable: false}
	result.Variables["port"] = pufferpanel.Variable{Type: pufferpanel.Type{Type: "integer"}, Value: 0, Display: "Port", Required: true, UserEditable: false}
	// Preserve the selected Docker image in the Puffer docker environment.
	if image != "" {
		result.Environment = pufferpanel.MetadataType{Type: "docker", Metadata: map[string]interface{}{"image": image, "networkName": "host"}}
	}
	if variables, ok := egg["variables"].([]interface{}); ok {
		for _, rawVariable := range variables {
			v, ok := rawVariable.(map[string]interface{})
			if !ok {
				continue
			}
			env := cast.ToString(v["env_variable"])
			if env == "" {
				continue
			}
			defaultValue := v["default_value"]
			if defaultValue == nil {
				defaultValue = ""
			}
			variableType := "string"
			if _, ok := defaultValue.(bool); ok {
				variableType = "boolean"
			}
			key := pterodactylVariableKey(env)
			// Pterodactyl rules are a pipe-delimited string, for example
			// "required|string" or "nullable|string". Only an explicit required
			// rule should make the corresponding panel field mandatory.
			rules := cast.ToString(v["rules"])
			required := false
			var options []pufferpanel.VariableOption
			for _, rule := range strings.Split(rules, "|") {
				rule = strings.TrimSpace(rule)
				if strings.HasPrefix(strings.ToLower(rule), "in:") && len(rule) > 3 {
					for _, choice := range strings.Split(rule[3:], ",") {
						options = append(options, pufferpanel.VariableOption{Value: choice, Display: choice})
					}
					continue
				}
				switch strings.ToLower(rule) {
				case "required":
					required = true
				case "boolean":
					variableType = "boolean"
					defaultValue = cast.ToBool(defaultValue)
				case "integer", "numeric":
					if n, err := cast.ToIntE(defaultValue); err == nil {
						variableType = "integer"
						defaultValue = n
					}
				}
			}
			userEditable := true
			if val, present := v["user_editable"]; present {
				userEditable = cast.ToBool(val)
			}
			variable := pufferpanel.Variable{Type: pufferpanel.Type{Type: variableType}, Value: defaultValue, Display: cast.ToString(v["name"]), Description: cast.ToString(v["description"]), Required: required, UserEditable: userEditable}
			if len(options) > 0 {
				// Keep the default when it matches a choice case-insensitively.
				variable.Type = pufferpanel.Type{Type: "option"}
				variable.Options = options
				for _, option := range options {
					if strings.EqualFold(option.Value.(string), cast.ToString(defaultValue)) {
						variable.Value = option.Value
					}
				}
			}
			result.Variables[key] = variable
			result.Execution.EnvironmentVariables[env] = "${" + key + "}"
		}
	}
	if script := pterodactylInstallScript(egg["scripts"]); script != "" {
		// Pterodactyl mounts the server at /mnt/server; PufferPanel's docker root is /pufferpanel.
		script = strings.ReplaceAll(script, "/mnt/server", "/pufferpanel")
		result.Installation = []pufferpanel.ConditionalMetadataType{
			{MetadataType: pufferpanel.MetadataType{Type: "writefile", Metadata: map[string]interface{}{"target": "pterodactyl-install.sh", "text": script}}},
			{MetadataType: pufferpanel.MetadataType{Type: "command", Metadata: map[string]interface{}{"commands": []string{"bash pterodactyl-install.sh"}}}},
		}
	}
	return result, nil
}

// pterodactylInstallScript returns scripts.installation.script, or "" if absent.
func pterodactylInstallScript(scripts interface{}) string {
	s, ok := scripts.(map[string]interface{})
	if !ok {
		return ""
	}
	installation, ok := s["installation"].(map[string]interface{})
	if !ok {
		return ""
	}
	return cast.ToString(installation["script"])
}

var pterodactylToken = regexp.MustCompile(`{{\s*([A-Za-z0-9_.]+)\s*}}`)
var slugInvalid = regexp.MustCompile(`[^a-z0-9-]+`)

func pterodactylTokens(command string) string {
	return pterodactylToken.ReplaceAllStringFunc(command, func(token string) string {
		match := pterodactylToken.FindStringSubmatch(token)
		return "${" + pterodactylVariableKey(match[1]) + "}"
	})
}
// pterodactylStop reads the legacy config.stop field, which is a JSON string.
func pterodactylStop(config interface{}) string {
	switch c := config.(type) {
	case string:
		var parsed map[string]interface{}
		if json.Unmarshal([]byte(c), &parsed) == nil {
			return cast.ToString(parsed["stop"])
		}
	case map[string]interface{}:
		return cast.ToString(c["stop"])
	}
	return ""
}

func pterodactylVariableKey(name string) string {
	// Handles forms such as {{server.build.env.NAME}} and {{server.build.default.port}}.
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	switch strings.ToUpper(name) {
	case "SERVER_PORT":
		return "port"
	case "SERVER_IP":
		return "ip"
	default:
		return strings.ToLower(name)
	}
}
func slug(value string) string {
	return strings.Trim(slugInvalid.ReplaceAllString(strings.ToLower(value), "-"), "-")
}
