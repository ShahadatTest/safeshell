// Package models lists the local models SafeShell can download. Every file is
// pinned by size and SHA-256, so what is downloaded is what was reviewed.
package models

// Model is a downloadable GGUF file.
type Model struct {
	ID    string // what the user types and config stores
	Title string
	Note  string
	URL   string
	File  string
	Size  int64
	SHA   string
}

// Default is the model used unless the user picks another.
const Default = "qwen2.5-coder-3b"

// Catalog is every local model on offer, smallest first.
var Catalog = []Model{
	{
		ID: "qwen2.5-coder-1.5b", Title: "Qwen2.5-Coder 1.5B", Note: "fastest, for slow machines",
		URL:  "https://huggingface.co/Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF/resolve/main/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf",
		File: "qwen2.5-coder-1.5b-instruct-q4_k_m.gguf", Size: 1117320768,
		SHA: "cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046",
	},
	{
		ID: "qwen2.5-coder-3b", Title: "Qwen2.5-Coder 3B", Note: "default: good balance of speed and accuracy",
		URL:  "https://huggingface.co/Qwen/Qwen2.5-Coder-3B-Instruct-GGUF/resolve/main/qwen2.5-coder-3b-instruct-q4_k_m.gguf",
		File: "qwen2.5-coder-3b-instruct-q4_k_m.gguf", Size: 2104932800,
		SHA: "724fb256bec1ff062b2f65e4569e871ad2e95ab2a3989723d1769c54294730b7",
	},
	{
		ID: "qwen3-4b", Title: "Qwen3 4B Instruct", Note: "newer general model, strong at following instructions",
		URL:  "https://huggingface.co/unsloth/Qwen3-4B-Instruct-2507-GGUF/resolve/main/Qwen3-4B-Instruct-2507-Q4_K_M.gguf",
		File: "Qwen3-4B-Instruct-2507-Q4_K_M.gguf", Size: 2497281120,
		SHA: "3605803b982cb64aead44f6c1b2ae36e3acdb41d8e46c8a94c6533bc4c67e597",
	},
	{
		ID: "qwen2.5-coder-7b", Title: "Qwen2.5-Coder 7B", Note: "most accurate; needs about 6 GB of free RAM",
		URL:  "https://huggingface.co/Qwen/Qwen2.5-Coder-7B-Instruct-GGUF/resolve/main/qwen2.5-coder-7b-instruct-q4_k_m.gguf",
		File: "qwen2.5-coder-7b-instruct-q4_k_m.gguf", Size: 4683073536,
		SHA: "509287f78cb4d4cf6b3843734733b914b2c158e43e22a7f4bf5e963800894d3c",
	},
}

// Find returns the catalog entry with the given id.
func Find(id string) (Model, bool) {
	for _, m := range Catalog {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}
