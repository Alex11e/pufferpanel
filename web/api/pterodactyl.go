package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/pufferpanel/pufferpanel/v3"
	"github.com/spf13/cast"
)

// pterodactylEggToTemplate intentionally converts only portable egg fields.
// Pterodactyl's install scripts use its own provisioning assumptions and are
// returned as a PufferPanel template without an install phase for review.
func pterodactylEggToTemplate(raw json.RawMessage) (*pufferpanel.Server, error) {
	var egg map[string]interface{}
	if err := json.Unmarshal(raw, &egg); err != nil { return nil, err }
	startup := cast.ToString(egg["startup"])
	if startup == "" { return nil, fmt.Errorf("the egg does not contain a startup command") }
	name := slug(cast.ToString(egg["name"]))
	if name == "" { name = "imported-egg" }
	image := ""
	if images, ok := egg["docker_images"].(map[string]interface{}); ok {
		for _, value := range images { image = cast.ToString(value); if image != "" { break } }
	}
	result := &pufferpanel.Server{
		Type: pufferpanel.Type{Type: "generic"}, Identifier: name, Display: cast.ToString(egg["name"]),
		Variables: map[string]pufferpanel.Variable{},
		Execution: pufferpanel.Execution{Command: pterodactylTokens(startup), StopCommand: cast.ToString(egg["stop"]), WorkingDirectory: ".", EnvironmentVariables: map[string]string{}},
		Environment: pufferpanel.MetadataType{Type: "docker"},
		SupportedEnvironments: []pufferpanel.MetadataType{{Type: "docker"}},
	}
	if result.Display == "" { result.Display = name }
	// These are Pterodactyl's built-in allocation placeholders, not egg
	// variables. Map them to PufferPanel's conventional allocation variables.
	result.Variables["ip"] = pufferpanel.Variable{Type: pufferpanel.Type{Type: "string"}, Value: "0.0.0.0", Display: "IP", Required: true, UserEditable: false}
	result.Variables["port"] = pufferpanel.Variable{Type: pufferpanel.Type{Type: "integer"}, Value: 0, Display: "Port", Required: true, UserEditable: false}
	// Preserve the selected Docker image in the Puffer docker environment.
	if image != "" { result.Environment = pufferpanel.MetadataType{Type: "docker", Metadata: map[string]interface{}{"image": image, "networkName": "host"}} }
	if variables, ok := egg["variables"].([]interface{}); ok {
		for _, rawVariable := range variables {
			v, ok := rawVariable.(map[string]interface{}); if !ok { continue }
			env := cast.ToString(v["env_variable"]); if env == "" { continue }
			defaultValue := v["default_value"]
			variableType := "string"
			if _, ok := defaultValue.(bool); ok { variableType = "boolean" }
			key := pterodactylVariableKey(env)
			result.Variables[key] = pufferpanel.Variable{Type: pufferpanel.Type{Type: variableType}, Value: defaultValue, Display: cast.ToString(v["name"]), Description: cast.ToString(v["description"]), Required: cast.ToBool(v["rules"] != "nullable"), UserEditable: true}
			result.Execution.EnvironmentVariables[env] = "${" + key + "}"
		}
	}
	return result, nil
}

var pterodactylToken = regexp.MustCompile(`{{\s*([A-Za-z0-9_]+)\s*}}`)
var slugInvalid = regexp.MustCompile(`[^a-z0-9-]+`)

func pterodactylTokens(command string) string {
	return pterodactylToken.ReplaceAllStringFunc(command, func(token string) string {
		match := pterodactylToken.FindStringSubmatch(token)
		return "${" + pterodactylVariableKey(match[1]) + "}"
	})
}
func pterodactylVariableKey(name string) string {
	switch strings.ToUpper(name) {
	case "SERVER_PORT":
		return "port"
	case "SERVER_IP":
		return "ip"
	default:
		return strings.ToLower(name)
	}
}
func slug(value string) string { return strings.Trim(slugInvalid.ReplaceAllString(strings.ToLower(value), "-"), "-") }
