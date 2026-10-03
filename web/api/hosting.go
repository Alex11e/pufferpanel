package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/scopes"
)

const (
	webHostingType = "webhosting"
	dbHostingType  = "dbhosting"
	discordBotType = "discordbot"
)

func registerHosting(g *gin.RouterGroup) {
	g.GET("/templates/:kind", middleware.RequiresPermission(scopes.ScopeServerCreate), getHostingTemplate)
	g.OPTIONS("/templates/:kind", response.CreateOptions("GET"))
}

func getHostingTemplate(c *gin.Context) {
	template := hostingTemplate(c.Param("kind"))
	if template == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, template)
}

func hostingVar(kind string, value any, display, desc string, required, editable bool) pufferpanel.Variable {
	return pufferpanel.Variable{Type: pufferpanel.Type{Type: kind}, Value: value, Display: display, Description: desc, Required: required, UserEditable: editable}
}

// hostingOption is a fixed-choice variable; the choices are what keeps ${version} from becoming an arbitrary image name.
func hostingOption(value string, display, desc string, choices ...string) pufferpanel.Variable {
	v := hostingVar("option", value, display, desc, true, true)
	for _, choice := range choices {
		v.Options = append(v.Options, pufferpanel.VariableOption{Value: choice, Display: choice})
	}
	return v
}

func hostingTemplate(kind string) *pufferpanel.Server {
	switch kind {
	case "web":
		return webHostingTemplate()
	case "discordbot":
		return discordBotTemplate()
	case "mariadb":
		return databaseTemplate("mariadb", "MariaDB", "mariadb", 3306, "/var/lib/mysql", "mariadbd", 15,
			[]string{"11.8", "11.4", "10.11"}, map[string]string{
				"MARIADB_ROOT_PASSWORD": "${root_password}",
				"MARIADB_DATABASE":      "${db_name}",
				"MARIADB_USER":          "${db_user}",
				"MARIADB_PASSWORD":      "${db_password}",
			}, true)
	case "postgres":
		return databaseTemplate("postgres", "PostgreSQL", "postgres", 5432, "/var/lib/postgresql/data", "postgres", 2,
			[]string{"17", "16", "15"}, map[string]string{
				"POSTGRES_USER":     "${db_user}",
				"POSTGRES_PASSWORD": "${db_password}",
				"POSTGRES_DB":       "${db_name}",
				// The data directory must be a subfolder of the mounted root.
				"PGDATA": "/var/lib/postgresql/data/pgdata",
			}, false)
	case "phpmyadmin":
		return phpMyAdminTemplate()
	}
	return nil
}

func discordBotTemplate() *pufferpanel.Server {
	token := hostingVar("string", "", "Discord bot token", "Kept private and provided to the bot as DISCORD_TOKEN.", true, false)
	token.Internal = true
	docker := pufferpanel.MetadataType{Type: "docker", Metadata: map[string]interface{}{
		"image":         "node:22-alpine",
		"containerRoot": "/home/node/app",
		"networkName":   "bridge",
	}}
	return &pufferpanel.Server{
		Type:       pufferpanel.Type{Type: discordBotType},
		Identifier: "discord-bot",
		Display:    "Discord bot (Node.js)",
		Variables: map[string]pufferpanel.Variable{
			"token": token,
		},
		Environment:           docker,
		SupportedEnvironments: []pufferpanel.MetadataType{docker},
		Installation: []pufferpanel.ConditionalMetadataType{
			{MetadataType: pufferpanel.MetadataType{Type: "writefile", Metadata: map[string]interface{}{
				"target": "package.json",
				"text":   "{\n  \"name\": \"pufferpanel-discord-bot\",\n  \"version\": \"1.0.0\",\n  \"private\": true,\n  \"main\": \"index.js\",\n  \"scripts\": { \"start\": \"node index.js\" },\n  \"dependencies\": { \"discord.js\": \"^14\" }\n}\n",
			}}},
			{MetadataType: pufferpanel.MetadataType{Type: "writefile", Metadata: map[string]interface{}{
				"target": "index.js",
				"text":   "const { Client, Events, GatewayIntentBits } = require('discord.js');\n\nconst token = process.env.DISCORD_TOKEN;\nif (!token) throw new Error('DISCORD_TOKEN is not configured');\n\nconst client = new Client({ intents: [GatewayIntentBits.Guilds] });\nclient.once(Events.ClientReady, readyClient => {\n  console.log(`Logged in as ${readyClient.user.tag}`);\n});\nclient.login(token);\n\nprocess.on('SIGTERM', () => { client.destroy(); process.exit(0); });\n",
			}}},
		},
		Execution: pufferpanel.Execution{
			Command:          `sh -c "npm install --omit=dev && node index.js"`,
			StopCode:         15,
			WorkingDirectory: "/home/node/app",
			EnvironmentVariables: map[string]string{
				"DISCORD_TOKEN": "${token}",
			},
		},
	}
}

// phpMyAdminTemplate uses host networking so it can reach databases bound to the node's loopback address.
func phpMyAdminTemplate() *pufferpanel.Server {
	engine := hostingVar("string", "phpmyadmin", "Engine", "", true, false)
	engine.Internal = true
	docker := pufferpanel.MetadataType{Type: "docker", Metadata: map[string]interface{}{
		"image":       "phpmyadmin:5-apache",
		"networkName": "host",
	}}
	return &pufferpanel.Server{
		Type:       pufferpanel.Type{Type: dbHostingType},
		Identifier: "phpmyadmin-hosting",
		Display:    "phpMyAdmin",
		Variables: map[string]pufferpanel.Variable{
			"port":     hostingVar("integer", 0, "Port", "Port of the web interface (allocated automatically).", true, false),
			"engine":   engine,
			"pma_host": hostingVar("string", "127.0.0.1", "Database host", "Database server phpMyAdmin connects to.", true, true),
			"pma_port": hostingVar("integer", 3306, "Database port", "Port of that database server.", true, true),
		},
		Environment:           docker,
		SupportedEnvironments: []pufferpanel.MetadataType{docker},
		Execution: pufferpanel.Execution{
			Command:          "apache2-foreground",
			StopCode:         15,
			WorkingDirectory: ".",
			EnvironmentVariables: map[string]string{
				"APACHE_PORT": "${port}",
				"PMA_HOST":    "${pma_host}",
				"PMA_PORT":    "${pma_port}",
			},
		},
	}
}

func hostingDocker(image, root string, containerPort int) pufferpanel.MetadataType {
	return pufferpanel.MetadataType{Type: "docker", Metadata: map[string]interface{}{
		"image":         image,
		"containerRoot": root,
		"networkName":   "bridge",
		"portBindings":  []string{"${ip}:${port}:" + strconv.Itoa(containerPort)},
	}}
}

const webWelcomePage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Your web hosting is ready</title></head>
<body style="font-family:sans-serif;max-width:40rem;margin:4rem auto">
<h1>Your web hosting is ready</h1>
<p>Upload your site to this folder with the file manager or SFTP and replace this page.</p>
</body></html>
`

func webHostingTemplate() *pufferpanel.Server {
	docker := hostingDocker("php:${php_version}-apache", "/var/www/html", 80)
	return &pufferpanel.Server{
		Type:       pufferpanel.Type{Type: webHostingType},
		Identifier: "web-hosting",
		Display:    "Web hosting (PHP + Apache)",
		Variables: map[string]pufferpanel.Variable{
			"ip":          hostingVar("string", "0.0.0.0", "IP", "Address the site listens on.", true, false),
			"port":        hostingVar("integer", 0, "Port", "Public port of the site (allocated automatically).", true, false),
			"php_version": hostingOption("8.3", "PHP version", "Applied the next time the site starts.", "8.4", "8.3", "8.2", "8.1"),
		},
		Environment:           docker,
		SupportedEnvironments: []pufferpanel.MetadataType{docker},
		Installation: []pufferpanel.ConditionalMetadataType{{
			MetadataType: pufferpanel.MetadataType{Type: "writefile", Metadata: map[string]interface{}{"target": "index.html", "text": webWelcomePage}},
		}},
		Execution: pufferpanel.Execution{
			Command:          "apache2-foreground",
			StopCode:         15,
			WorkingDirectory: ".",
		},
	}
}

func databaseTemplate(id, display, image string, port int, root, command string, stopCode int, versions []string, env map[string]string, hasRoot bool) *pufferpanel.Server {
	docker := hostingDocker(image+":${version}", root, port)
	engine := hostingVar("string", id, "Engine", "", true, false)
	engine.Internal = true
	variables := map[string]pufferpanel.Variable{
		"ip":          hostingVar("string", "0.0.0.0", "IP", "Address the database listens on.", true, false),
		"port":        hostingVar("integer", 0, "Port", "Public port of the database (allocated automatically).", true, false),
		"engine":      engine,
		"version":     hostingOption(versions[0], "Version", "Applied the next time the database starts; downgrading is not supported.", versions...),
		"db_name":     hostingVar("string", "app", "Database name", "Created on first start only.", true, false),
		"db_user":     hostingVar("string", "app", "Database user", "Created on first start only.", true, false),
		"db_password": hostingVar("string", "", "Database password", "Used on first start only; change it afterwards with SQL.", true, false),
	}
	if hasRoot {
		variables["root_password"] = hostingVar("string", "", "Root password", "Used on first start only; change it afterwards with SQL.", true, false)
	}
	return &pufferpanel.Server{
		Type:                  pufferpanel.Type{Type: dbHostingType},
		Identifier:            id + "-hosting",
		Display:               "Database hosting (" + display + ")",
		Variables:             variables,
		Environment:           docker,
		SupportedEnvironments: []pufferpanel.MetadataType{docker},
		Execution: pufferpanel.Execution{
			Command:              command,
			StopCode:             stopCode,
			WorkingDirectory:     ".",
			EnvironmentVariables: env,
		},
	}
}
