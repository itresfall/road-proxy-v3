package i18n

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const DefaultLanguage = "en"

type Translator struct {
	language string
	fallback map[string]string
	messages map[string]string

	mu      sync.Mutex
	missing map[string]struct{}
}

func New(language string, fallback, messages map[string]string) *Translator {
	if fallback == nil {
		fallback = map[string]string{}
	}
	if messages == nil {
		messages = map[string]string{}
	}
	return &Translator{
		language: NormalizeLanguage(language),
		fallback: fallback,
		messages: messages,
		missing:  map[string]struct{}{},
	}
}

func Load(dir, language string) (*Translator, error) {
	lang := NormalizeLanguage(language)

	fallback, err := loadCatalog(filepath.Join(dir, DefaultLanguage+".json"))
	if err != nil {
		return nil, err
	}

	messages := fallback
	if lang != DefaultLanguage {
		loaded, loadErr := loadCatalog(filepath.Join(dir, lang+".json"))
		if loadErr == nil {
			messages = loaded
		} else {
			messages = map[string]string{}
		}
	}

	return New(lang, fallback, messages), nil
}

func (t *Translator) Language() string {
	if t == nil {
		return DefaultLanguage
	}
	return t.language
}

func (t *Translator) T(key string) string {
	if t == nil {
		return key
	}

	msg, ok := t.messages[key]
	if !ok {
		msg, ok = t.fallback[key]
		if !ok {
			msg = key
		}
		t.recordMissing(key)
	}

	return msg
}

func (t *Translator) MissingKeys() []string {
	if t == nil {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	keys := make([]string, 0, len(t.missing))
	for key := range t.missing {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func NormalizeLanguage(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "tr", "tr-tr", "turkish":
		return "tr"
	case "", "en", "en-us", "en-gb", "eng", "english":
		return DefaultLanguage
	default:
		return DefaultLanguage
	}
}

func loadCatalog(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read locale %q: %w", path, err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	var catalog map[string]string
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("parse locale %q: %w", path, err)
	}
	if catalog == nil {
		catalog = map[string]string{}
	}
	return catalog, nil
}

func (t *Translator) recordMissing(key string) {
	t.mu.Lock()
	t.missing[key] = struct{}{}
	t.mu.Unlock()
}
