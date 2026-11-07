// Package ai defines the Backend interface (Complete with a JSON schema) and the
// guard rules: AI may only raise risk, output is schema-constrained, file names are
// untrusted data, paths are redacted before leaving the machine.
// Subpackages: local (OpenAI-compatible endpoint such as Ollama), cloud, none.
package ai
