// SPDX-License-Identifier: AGPL-3.0-or-later

package preflight

import "strings"

// Family is the kind of distribution, which decides the package manager and the package names.
type Family string

// The families with their own install command.
const (
	FamilyArch   Family = "arch"
	FamilyDebian Family = "debian" // Debian, Ubuntu and what is built on them
	FamilyFedora Family = "fedora" // Fedora, RHEL and what is built on them
	FamilyOther  Family = ""
)

// ParseOSRelease reads the text of /etc/os-release and says which family the system belongs to: by ID first, then by
// each word of ID_LIKE (CachyOS is ID=cachyos with ID_LIKE=arch).
func ParseOSRelease(text string) Family {
	values := osReleaseValues(text)
	candidates := append([]string{values["ID"]}, strings.Fields(values["ID_LIKE"])...)
	for _, id := range candidates {
		if family := familyOf(id); family != FamilyOther {
			return family
		}
	}
	return FamilyOther
}

// osReleaseValues reads KEY=value lines; a value may be in single or double quotes.
func osReleaseValues(text string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || strings.HasPrefix(key, "#") {
			continue
		}
		values[key] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return values
}

func familyOf(id string) Family {
	switch strings.ToLower(id) {
	case "arch", "cachyos", "manjaro", "endeavouros", "garuda", "artix":
		return FamilyArch
	case "debian", "ubuntu", "linuxmint", "pop", "raspbian", "elementary", "zorin":
		return FamilyDebian
	case "fedora", "rhel", "centos", "rocky", "almalinux", "nobara":
		return FamilyFedora
	}
	return FamilyOther
}

// InstallCommand is the command that installs what the window needs on a system of this family.
func InstallCommand(family Family) string {
	switch family {
	case FamilyArch:
		return "sudo pacman -S gtk4 webkitgtk-6.0 libsoup3 bubblewrap xdg-dbus-proxy"
	case FamilyDebian:
		return "sudo apt install libgtk-4-1 libwebkitgtk-6.0-4 libsoup-3.0-0 bubblewrap xdg-dbus-proxy" +
			" (needs Ubuntu 24.04 or Debian 13 or later)"
	case FamilyFedora:
		return "sudo dnf install gtk4 webkitgtk6.0 libsoup3 bubblewrap xdg-dbus-proxy"
	}
	return "install GTK 4, WebKitGTK 6.0, libsoup 3, bubblewrap and xdg-dbus-proxy with your package manager"
}
