package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/DreamCats/dev-connect/internal/config"
)

type Entry struct {
	Count    int    `json:"count"`
	LastUsed string `json:"last_used"`
}

func path() string { return filepath.Join(config.ConfigDir(), "stats.json") }

func Load() map[string]Entry {
	data := map[string]Entry{}
	raw, err := os.ReadFile(path())
	if err != nil {
		return data
	}
	_ = json.Unmarshal(raw, &data)
	return data
}

func Record(command string) {
	data := Load()
	entry := data[command]
	entry.Count++
	entry.LastUsed = time.Now().UTC().Format(time.RFC3339Nano)
	data[command] = entry
	_ = os.MkdirAll(config.ConfigDir(), 0o755)
	raw, _ := json.MarshalIndent(data, "", "  ")
	_ = os.WriteFile(path(), raw, 0o644)
}
