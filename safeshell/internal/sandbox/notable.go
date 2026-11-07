package sandbox

import (
	"path/filepath"
	"strings"
)

var notableDirs = map[string]bool{".git": true, ".ssh": true, ".gnupg": true, ".aws": true, ".kube": true}

var notableNames = map[string]bool{
	"id_rsa": true, "id_ed25519": true, "id_ecdsa": true, ".env": true, ".netrc": true,
	"wallet.dat": true, ".bashrc": true, ".zshrc": true, ".profile": true, "authorized_keys": true,
}

var notableExts = map[string]bool{
	// secrets and databases
	".pem": true, ".key": true, ".kdbx": true, ".sqlite": true, ".sqlite3": true, ".db": true,
	// documents people write by hand
	".pdf": true, ".doc": true, ".docx": true, ".odt": true, ".xls": true, ".xlsx": true, ".ods": true,
	".ppt": true, ".pptx": true, ".tex": true, ".ipynb": true, ".md": true,
}

// IsNotable reports whether a path (relative or absolute) looks like something
// a person would be upset to lose: git history, keys, databases, documents.
func IsNotable(path string) bool {
	if path == "" || path == "." {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if notableDirs[part] {
			return true
		}
	}
	base := filepath.Base(path)
	return notableNames[base] || notableExts[strings.ToLower(filepath.Ext(base))]
}
