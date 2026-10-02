package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/scopes"
)

const (
	vpsType  = "hypervm"
	vpsImage = "ghcr.io/hypernestz/hypervm:main"
)

func registerVps(g *gin.RouterGroup) {
	g.GET("/template", middleware.RequiresPermission(scopes.ScopeServerCreate), getVpsTemplate)
	g.OPTIONS("/template", response.CreateOptions("GET"))
}

func getVpsTemplate(c *gin.Context) {
	c.JSON(http.StatusOK, vpsTemplate())
}

// vpsStartScript is rewritten on every start so template fixes reach existing VPSes.
// User-editable values are validated and passed to QEMU as single arguments (never re-split by the shell).
const vpsStartScript = `#!/bin/bash
set -u
on() { [ "$1" = "1" ] || [ "$1" = "true" ]; }
esc() { printf '%s' "${1//,/,,}"; }
fail() { echo "$1"; exit 1; }

# QEMU needs a writable temp directory; the container filesystem is read-only for this user.
mkdir -p tmp && export TMPDIR="$PWD/tmp"

[[ "${RAM:-}" =~ ^[0-9]+$ ]] && [ "$RAM" -ge 256 ] || fail "RAM must be a number of at least 256 (MB)"
[[ "${VNC_DISPLAY:-}" =~ ^[0-9]+$ ]] || fail "VNC display must be a number"
[ -n "${VNC_PASSWORD:-}" ] || fail "A VNC password is required"
[ -n "${DISK_FILE:-}" ] || fail "Disk file is required"
fetch() { curl -fsSL -o "$2" "$1"; }
if [ ! -f "$DISK_FILE" ]; then
  [[ "${DISK_SIZE:-}" =~ ^[0-9]+$ ]] && [ "$DISK_SIZE" -ge 1 ] || fail "Disk size must be a number of GB"
  echo "Creating ${DISK_SIZE} GB disk image..."
  qemu-img create -f qcow2 "$DISK_FILE" "${DISK_SIZE}G" || fail "Could not create the disk image"
fi
# The install step normally downloads these; fetch again if they are missing.
[ -f OVMF.fd ] || fetch https://github.com/hypernestz/hypervm/releases/download/1.0/OVMF.fd OVMF.fd || fail "Could not download OVMF.fd"
[ -f bios.bin ] || fetch https://github.com/hypernestz/hypervm/releases/download/1.0/bios.bin bios.bin || fail "Could not download bios.bin"
fwd='hostfwd=(tcp|udp):[0-9.]*:[0-9]+-[0-9.]*:[0-9]+'
if [ -n "${FORWARD_PORTS:-}" ] && ! [[ "$FORWARD_PORTS" =~ ^$fwd(,$fwd)*$ ]]; then
  fail "Invalid forwarded ports, use hostfwd=tcp::HOST-:GUEST (comma separated)"
fi

args=(-device qemu-xhci -m "$RAM")
if on "${USE_KVM:-0}"; then args+=(-enable-kvm -cpu host); else args+=(-accel "tcg,thread=multi,tb-size=128,split-wx=on"); fi
args+=(-smp "sockets=1,cores=$(nproc),threads=1")
[ -n "${ISO_FILE:-}" ] && args+=(-drive "file=$(esc "$ISO_FILE"),media=cdrom")
# Our own images are qcow2; anything else is treated as raw so a guest cannot make QEMU re-probe its disk as another format.
diskfmt=raw
[ "$(head -c 3 "$DISK_FILE")" = "QFI" ] && diskfmt=qcow2
drive="file=$(esc "$DISK_FILE"),format=$diskfmt"
if on "${USE_VIRTIO:-0}"; then drive+=",aio=native,cache.direct=on,if=virtio"; fi
args+=(-drive "$drive")
[ -n "${SHARED_DIR:-}" ] && args+=(-drive "file=fat:rw:$(esc "$SHARED_DIR")")
if on "${USE_GPU:-0}"; then args+=(-device virtio-gpu-gl-pci,max_hostmem=256M -display egl-headless); else args+=(-vga std); fi
if on "${USE_VIRTIO_NET:-0}"; then
  args+=(-device virtio-net-pci,netdev=n0 -netdev "user,id=n0${FORWARD_PORTS:+,$FORWARD_PORTS}")
else
  args+=(-net nic -net "user${FORWARD_PORTS:+,$FORWARD_PORTS}")
fi
args+=(-device usb-tablet -boot "c,menu=on,splash-time=5000")
if on "${USE_UEFI:-0}"; then
  args+=(-pflash OVMF.fd)
else
  args+=(-bios bios.bin)
fi
args+=(-vnc "0.0.0.0:${VNC_DISPLAY},password" -monitor stdio)

# Browser console (noVNC) shipped with the image; it ends together with QEMU when the container stops.
if [ -x /opt/novnc/utils/websockify/run ] && [[ "${SERVER_PORT:-}" =~ ^[0-9]+$ ]] && [ "$SERVER_PORT" -gt 0 ]; then
  /opt/novnc/utils/websockify/run --web /opt/novnc "0.0.0.0:${SERVER_PORT}" "localhost:$((5900 + VNC_DISPLAY))" >/dev/null 2>&1 &
fi

# The monitor reads the password first, then the panel console input.
exec qemu-system-x86_64 "${args[@]}" < <({ printf 'change vnc password\n%s\n' "$VNC_PASSWORD"; cat; })
`

type vpsVariableDef struct {
	key, env, display, desc string
	kind                    string
	value                   any
	required, editable      bool
}

var vpsVariables = []vpsVariableDef{
	{"port", "SERVER_PORT", "Web console port", "Port of the browser (noVNC) console; allocated automatically.", "integer", 0, true, false},
	{"ram", "RAM", "Memory (MB)", "Total RAM for the VPS in MB (minimum 256).", "integer", 2048, true, true},
	{"disk_size", "DISK_SIZE", "Disk size (GB)", "Size of the virtual disk. Only used when the disk is created during install.", "integer", 20, true, false},
	{"disk_file", "DISK_FILE", "Disk file", "Name of the virtual disk image.", "string", "disk.qcow2", true, true},
	{"use_kvm", "USE_KVM", "KVM acceleration", "Hardware acceleration. Needs /dev/kvm in the container, otherwise the VPS uses slower software emulation.", "boolean", false, false, true},
	{"use_uefi", "USE_UEFI", "UEFI boot", "UEFI (OVMF) instead of legacy BIOS.", "boolean", true, false, true},
	{"use_virtio", "USE_VIRTIO", "VirtIO disk", "Faster disk I/O; the guest OS needs VirtIO drivers.", "boolean", false, false, true},
	{"use_virtio_net", "USE_VIRTIO_NET", "VirtIO network", "Faster networking; the guest OS needs VirtIO drivers.", "boolean", false, false, true},
	{"use_gpu", "USE_GPU", "VirtIO GPU", "GPU acceleration (needs a host GPU), otherwise standard VGA.", "boolean", false, false, true},
	{"iso_file", "ISO_FILE", "ISO file", "Boot ISO inside the server files (leave empty to boot from disk).", "string", "netboot.xyz.iso", false, true},
	{"shared_dir", "SHARED_DIR", "Shared directory", "Optional folder exposed to the VPS as a FAT drive.", "string", "", false, true},
	{"forward_ports", "FORWARD_PORTS", "Forwarded ports", "Format hostfwd=tcp::HOST-:GUEST, comma separated.", "string", "", false, true},
	{"vnc_display", "VNC_DISPLAY", "VNC display", "VNC listens on port 5900 + this number. Must be unique on the node.", "integer", 1, true, false},
	{"vnc_password", "VNC_PASSWORD", "VNC password", "Password for the VNC console (QEMU uses only the first 8 characters).", "string", "", true, true},
}

// vpsTemplate is the built-in HyperVM (QEMU) template used by the VPS section.
func vpsTemplate() *pufferpanel.Server {
	variables := map[string]pufferpanel.Variable{}
	env := map[string]string{}
	for _, v := range vpsVariables {
		variables[v.key] = pufferpanel.Variable{
			Type: pufferpanel.Type{Type: v.kind}, Value: v.value, Display: v.display,
			Description: v.desc, Required: v.required, UserEditable: v.editable,
		}
		env[v.env] = "${" + v.key + "}"
	}
	docker := pufferpanel.MetadataType{Type: "docker", Metadata: map[string]interface{}{"image": vpsImage, "networkName": "host"}}
	step := func(kind string, args map[string]interface{}) pufferpanel.ConditionalMetadataType {
		return pufferpanel.ConditionalMetadataType{MetadataType: pufferpanel.MetadataType{Type: kind, Metadata: args}}
	}
	return &pufferpanel.Server{
		Type:                  pufferpanel.Type{Type: vpsType},
		Identifier:            "hypervm",
		Display:               "VPS (HyperVM)",
		Variables:             variables,
		Environment:           docker,
		SupportedEnvironments: []pufferpanel.MetadataType{docker},
		// Downloads run in the daemon, so installing needs no container or image pull.
		Installation: []pufferpanel.ConditionalMetadataType{
			step("download", map[string]interface{}{"files": []string{
				"https://github.com/hypernestz/hypervm/releases/download/1.0/OVMF.fd",
				"https://github.com/hypernestz/hypervm/releases/download/1.0/bios.bin",
				"https://boot.netboot.xyz/ipxe/netboot.xyz.iso",
			}}),
		},
		Execution: pufferpanel.Execution{
			Command:              "bash start.sh",
			StopCommand:          "system_powerdown",
			WorkingDirectory:     ".",
			EnvironmentVariables: env,
			PreExecution: []pufferpanel.ConditionalMetadataType{
				step("writefile", map[string]interface{}{"target": "start.sh", "text": vpsStartScript}),
			},
		},
	}
}
