// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package version

import (
	"encoding/xml"
	"os"
	"path/filepath"
)

const appID = "io.github.ashbuk.dabri"

// Version is set at build time via ldflags. Packaged builds (e.g. Flatpak) that
// don't stamp it fall back to the installed AppStream metainfo at startup.
var Version = "dev"

func init() {
	if Version == "dev" {
		if v := metainfoVersion(); v != "" {
			Version = v
		}
	}
}

// metainfoVersion returns the current release version from the installed
// AppStream metainfo (releases are listed newest-first), or "" if not found.
func metainfoVersion() string {
	dirs := append(filepath.SplitList(os.Getenv("XDG_DATA_DIRS")), "/app/share", "/usr/share")

	var doc struct {
		Release []struct {
			Version string `xml:"version,attr"`
		} `xml:"releases>release"`
	}
	for _, dir := range dirs {
		data, err := os.ReadFile(filepath.Join(dir, "metainfo", appID+".metainfo.xml"))
		if err == nil && xml.Unmarshal(data, &doc) == nil && len(doc.Release) > 0 {
			return doc.Release[0].Version
		}
	}
	return ""
}
