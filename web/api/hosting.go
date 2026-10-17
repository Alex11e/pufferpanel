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
	webHostingType  = "webhosting"
	dbHostingType   = "dbhosting"
	discordBotType  = "discordbot"
	jellyfinType    = "jellyfin"
	staticMusicType = "staticduomusic"
	storageType     = "storage"
	gibbonType      = "gibbon"
	sambaType       = "samba"
)

func registerHosting(g *gin.RouterGroup) {
	g.GET("/templates/:kind", middleware.RequiresPermission(scopes.ScopeServerCreate), getHostingTemplate)
	g.GET("/templates/storage-samba", middleware.RequiresPermission(scopes.ScopeAdmin), getHostingTemplate)
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
	case "jellyfin":
		return jellyfinTemplate()
	case "staticduomusic":
		return staticduomusicTemplate()
	case "storage-files":
		return storageFilesTemplate()
	case "storage-nextcloud":
		return nextcloudTemplate()
	case "storage-samba":
		return sambaTemplate()
	case "gibbon":
		return gibbonTemplate()
	case "discordbot":
		return discordBotTemplate("node")
	case "discordbot-python":
		return discordBotTemplate("python")
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

func sambaTemplate() *pufferpanel.Server {
	docker := hostingDocker("ghcr.io/servercontainers/samba:smbd-only-latest", "/shares", 445)
	docker.Metadata["config"] = map[string]interface{}{"user": "0:0"}
	return &pufferpanel.Server{
		Type:       pufferpanel.Type{Type: sambaType},
		Identifier: "samba-storage",
		Display:    "SMB fájlmegosztás",
		Variables: map[string]pufferpanel.Variable{
			"ip":           hostingVar("string", "0.0.0.0", "IP", "Address the SMB service listens on.", true, false),
			"port":         hostingVar("integer", 445, "SMB port", "Fixed SMB port on the node.", true, false),
			"smb_password": {Type: pufferpanel.Type{Type: "string"}, Value: "", Display: "SMB password", Description: "Password for the private SMB share account.", Required: true, Internal: true},
		},
		Environment:           docker,
		SupportedEnvironments: []pufferpanel.MetadataType{docker},
		Execution: pufferpanel.Execution{
			WorkingDirectory: "/shares",
			EnvironmentVariables: map[string]string{
				"ACCOUNT_puffer":            "${smb_password}",
				"SAMBA_VOLUME_CONFIG_files": "path = /shares; browseable = yes; read only = no; guest ok = no; valid users = puffer",
			},
		},
	}
}

func jellyfinTemplate() *pufferpanel.Server {
	docker := hostingDocker("jellyfin/jellyfin:10", "/config", 8096)
	return &pufferpanel.Server{
		Type:       pufferpanel.Type{Type: jellyfinType},
		Identifier: "jellyfin",
		Display:    "Jellyfin",
		Variables: map[string]pufferpanel.Variable{
			"ip":   hostingVar("string", "0.0.0.0", "IP", "Address the media server listens on.", true, false),
			"port": hostingVar("integer", 0, "Port", "Public port of the Jellyfin web interface (allocated automatically).", true, false),
		},
		Environment:           docker,
		SupportedEnvironments: []pufferpanel.MetadataType{docker},
		Installation: []pufferpanel.ConditionalMetadataType{{
			MetadataType: pufferpanel.MetadataType{Type: "mkdir", Metadata: map[string]interface{}{"target": "media"}},
		}},
		Execution: pufferpanel.Execution{WorkingDirectory: "/config"},
	}
}

func staticduomusicTemplate() *pufferpanel.Server {
	docker := hostingDocker("node:22-alpine", "/app", 4000)
	return &pufferpanel.Server{
		Type:       pufferpanel.Type{Type: staticMusicType},
		Identifier: "staticduomusic",
		Display:    "HANGTÁR (Staticduomusic)",
		Variables: map[string]pufferpanel.Variable{
			"ip":   hostingVar("string", "0.0.0.0", "IP", "Address the music server listens on.", true, false),
			"port": hostingVar("integer", 0, "Port", "Public port of the music server (allocated automatically).", true, false),
		},
		Environment:           docker,
		SupportedEnvironments: []pufferpanel.MetadataType{docker},
		Installation: []pufferpanel.ConditionalMetadataType{{
			MetadataType: pufferpanel.MetadataType{Type: "command", Metadata: map[string]interface{}{
				"commands": []string{`sh -c "wget -qO /tmp/staticduomusic.tar.gz https://codeload.github.com/Alex11e/Staticduomusic/tar.gz/refs/heads/main && tar -xzf /tmp/staticduomusic.tar.gz --strip-components=1 -C /app && npm ci && npm run build && npm prune --omit=dev"`},
			}},
		}},
		Execution: pufferpanel.Execution{
			Command:          "node server/index.js",
			StopCode:         15,
			WorkingDirectory: "/app",
			EnvironmentVariables: map[string]string{
				"HOST": "0.0.0.0",
				"PORT": "4000",
			},
		},
	}
}

func storageFilesTemplate() *pufferpanel.Server {
	docker := pufferpanel.MetadataType{Type: "docker", Metadata: map[string]interface{}{
		"image":         "alpine:3.22",
		"containerRoot": "/data",
		"networkName":   "none",
	}}
	return &pufferpanel.Server{
		Type:                  pufferpanel.Type{Type: storageType},
		Identifier:            "storage-files",
		Display:               "Fájltár (Fájlok + SFTP)",
		Variables:             map[string]pufferpanel.Variable{},
		Environment:           docker,
		SupportedEnvironments: []pufferpanel.MetadataType{docker},
		Execution: pufferpanel.Execution{
			Command:          `sh -c "while true; do sleep 3600; done"`,
			WorkingDirectory: "/data",
			StopCode:         15,
		},
	}
}

func nextcloudTemplate() *pufferpanel.Server {
	docker := hostingDocker("nextcloud:apache", "/var/www/html", 80)
	return &pufferpanel.Server{
		Type:       pufferpanel.Type{Type: storageType},
		Identifier: "nextcloud",
		Display:    "Nextcloud (SQLite)",
		Variables: map[string]pufferpanel.Variable{
			"ip":   hostingVar("string", "0.0.0.0", "IP", "Address the Nextcloud web interface listens on.", true, false),
			"port": hostingVar("integer", 0, "Port", "Public port of the Nextcloud web interface (allocated automatically).", true, false),
		},
		Environment:           docker,
		SupportedEnvironments: []pufferpanel.MetadataType{docker},
		Execution:             pufferpanel.Execution{WorkingDirectory: "/var/www/html"},
	}
}

func gibbonTemplate() *pufferpanel.Server {
	docker := hostingDocker("webdevops/php-apache:8.3", "/var/www/html", 80)
	step := func(kind string, metadata map[string]interface{}) pufferpanel.ConditionalMetadataType {
		return pufferpanel.ConditionalMetadataType{MetadataType: pufferpanel.MetadataType{Type: kind, Metadata: metadata}}
	}
	variables := map[string]pufferpanel.Variable{
		"ip":          hostingVar("string", "0.0.0.0", "IP", "Address the Gibbon web interface listens on.", true, false),
		"port":        hostingVar("integer", 0, "Port", "Public port of the Gibbon web interface (allocated automatically).", true, false),
		"db_host":     hostingVar("string", "", "Database host", "Private MariaDB connection, provisioned with this Gibbon service.", true, false),
		"db_port":     hostingVar("integer", 0, "Database port", "Private MariaDB connection port.", true, false),
		"db_name":     hostingVar("string", "gibbon", "Database name", "Private MariaDB database name.", true, false),
		"db_user":     hostingVar("string", "gibbon", "Database user", "Private MariaDB user.", true, false),
		"db_password": hostingVar("string", "", "Database password", "Private MariaDB password.", true, false),
	}
	for _, key := range []string{"db_host", "db_port", "db_name", "db_user", "db_password"} {
		variable := variables[key]
		variable.Internal = true
		variables[key] = variable
	}
	return &pufferpanel.Server{
		Type:                  pufferpanel.Type{Type: gibbonType},
		Identifier:            "gibbon",
		Display:               "Gibbon EDU",
		Variables:             variables,
		Environment:           docker,
		SupportedEnvironments: []pufferpanel.MetadataType{docker},
		Installation: []pufferpanel.ConditionalMetadataType{
			step("download", map[string]interface{}{"files": []string{"https://github.com/GibbonEdu/core/archive/refs/tags/v31.0.00.tar.gz"}}),
			step("extract", map[string]interface{}{"source": "v31.0.00.tar.gz", "destination": "."}),
		},
		Execution: pufferpanel.Execution{
			Command:          `sh -c "cd /var/www/html/core-31.0.00 && until php -r '$fp = @fsockopen(getenv(\"DB_HOST\"), (int)getenv(\"DB_PORT\"), $errno, $error, 2); if ($fp) { fclose($fp); exit(0); } exit(1);'; do sleep 2; done && COMPOSER_PROCESS_TIMEOUT=0 composer install --no-dev --no-interaction --prefer-dist && exec supervisord"`,
			WorkingDirectory: "/var/www/html/core-31.0.00",
			EnvironmentVariables: map[string]string{
				"WEB_DOCUMENT_ROOT": "/var/www/html/core-31.0.00",
				"DB_HOST":           "${db_host}",
				"DB_PORT":           "${db_port}",
				"DB_NAME":           "${db_name}",
				"DB_USERNAME":       "${db_user}",
				"DB_PASSWORD":       "${db_password}",
			},
		},
	}
}

func discordBotTemplate(language string) *pufferpanel.Server {
	token := hostingVar("string", "", "Discord bot token", "Kept private and provided to the bot as DISCORD_TOKEN.", true, false)
	token.Internal = true
	image := "node:22-alpine"
	root := "/home/node/app"
	display := "Discord bot (Node.js)"
	identifier := "discord-bot"
	installation := []pufferpanel.ConditionalMetadataType{
		{MetadataType: pufferpanel.MetadataType{Type: "writefile", Metadata: map[string]interface{}{
			"target": "package.json",
			"text":   "{\n  \"name\": \"pufferpanel-discord-bot\",\n  \"version\": \"1.0.0\",\n  \"private\": true,\n  \"main\": \"index.js\",\n  \"scripts\": { \"start\": \"node index.js\" },\n  \"dependencies\": { \"discord.js\": \"^14\" }\n}\n",
		}}},
		{MetadataType: pufferpanel.MetadataType{Type: "writefile", Metadata: map[string]interface{}{
			"target": "index.js",
			"text":   "const { Client, Events, GatewayIntentBits } = require('discord.js');\n\nconst token = process.env.DISCORD_TOKEN;\nif (!token) throw new Error('DISCORD_TOKEN is not configured');\n\nconst client = new Client({ intents: [GatewayIntentBits.Guilds] });\nclient.once(Events.ClientReady, readyClient => {\n  console.log(`Logged in as ${readyClient.user.tag}`);\n});\nclient.login(token);\n\nprocess.on('SIGTERM', () => { client.destroy(); process.exit(0); });\n",
		}}},
	}
	command := `sh -c "npm install --omit=dev && node index.js"`
	if language == "python" {
		image = "python:3.12-alpine"
		root = "/home/bot/app"
		display = "Discord bot (Python)"
		identifier = "discord-bot-python"
		installation = []pufferpanel.ConditionalMetadataType{
			{MetadataType: pufferpanel.MetadataType{Type: "writefile", Metadata: map[string]interface{}{
				"target": "requirements.txt",
				"text":   "discord.py>=2,<3\n",
			}}},
			{MetadataType: pufferpanel.MetadataType{Type: "writefile", Metadata: map[string]interface{}{
				"target": "main.py",
				"text":   "import os\nimport discord\n\ntoken = os.environ.get(\"DISCORD_TOKEN\")\nif not token:\n    raise RuntimeError(\"DISCORD_TOKEN is not configured\")\n\nintents = discord.Intents.default()\nclient = discord.Client(intents=intents)\n\n@client.event\nasync def on_ready():\n    print(f\"Logged in as {client.user}\")\n\nclient.run(token)\n",
			}}},
		}
		command = "sh -c \"pip install --no-cache-dir -r requirements.txt && python main.py\""
	}
	docker := pufferpanel.MetadataType{Type: "docker", Metadata: map[string]interface{}{
		"image":         image,
		"containerRoot": root,
		"networkName":   "bridge",
	}}
	return &pufferpanel.Server{
		Type:       pufferpanel.Type{Type: discordBotType},
		Identifier: identifier,
		Display:    display,
		Variables: map[string]pufferpanel.Variable{
			"token": token,
		},
		Environment:           docker,
		SupportedEnvironments: []pufferpanel.MetadataType{docker},
		Installation:          installation,
		Execution: pufferpanel.Execution{
			Command:          command,
			StopCode:         15,
			WorkingDirectory: root,
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
	databaseUser := hostingVar("string", "", "Database user", "Private database account used by phpMyAdmin.", true, false)
	databaseUser.Internal = true
	databasePassword := hostingVar("string", "", "Database password", "Private password used by phpMyAdmin.", true, false)
	databasePassword.Internal = true
	docker := pufferpanel.MetadataType{Type: "docker", Metadata: map[string]interface{}{
		"image":       "phpmyadmin:5-apache",
		"networkName": "host",
	}}
	return &pufferpanel.Server{
		Type:       pufferpanel.Type{Type: dbHostingType},
		Identifier: "phpmyadmin-hosting",
		Display:    "phpMyAdmin",
		Variables: map[string]pufferpanel.Variable{
			"port":         hostingVar("integer", 0, "Port", "Port of the web interface (allocated automatically).", true, false),
			"engine":       engine,
			"pma_host":     hostingVar("string", "127.0.0.1", "Database host", "Database server phpMyAdmin connects to.", true, true),
			"pma_port":     hostingVar("integer", 3306, "Database port", "Port of that database server.", true, true),
			"pma_database": hostingVar("string", "app", "Database name", "Database selected when phpMyAdmin connects.", true, true),
			"pma_user":     databaseUser,
			"pma_password": databasePassword,
		},
		Environment:           docker,
		SupportedEnvironments: []pufferpanel.MetadataType{docker},
		Execution: pufferpanel.Execution{
			Command:          "apache2-foreground",
			StopCode:         15,
			WorkingDirectory: ".",
			EnvironmentVariables: map[string]string{
				"APACHE_PORT":  "${port}",
				"PMA_HOST":     "${pma_host}",
				"PMA_PORT":     "${pma_port}",
				"PMA_DATABASE": "${pma_database}",
				"PMA_USER":     "${pma_user}",
				"PMA_PASSWORD": "${pma_password}",
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
