package synowake

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
)

//go:embed translations.json
var translationJSON []byte

var english = func() map[string]string {
	var result map[string]string
	if err := json.Unmarshal(translationJSON, &result); err != nil {
		panic(err)
	}
	return result
}()

var parameterPattern = regexp.MustCompile(`\{(\d+)\}`)

func normalizeLanguage(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "ger" || value == "german" || value == "de" || strings.HasPrefix(value, "de-") || strings.HasPrefix(value, "de_") {
		return "de"
	}
	return "en"
}

func systemLanguage() string { return normalizeLanguage(os.Getenv("SYNOPKG_DSM_LANGUAGE")) }

func formatMessage(key, language string, args ...string) string {
	pattern := key
	if language == "en" {
		if translated, ok := english[key]; ok {
			pattern = translated
		}
	}
	return parameterPattern.ReplaceAllStringFunc(pattern, func(value string) string {
		var index int
		fmt.Sscanf(value, "{%d}", &index)
		if index < len(args) {
			return args[index]
		}
		return value
	})
}

// Legacy stored log messages and wrapped Go errors use known source prefixes.
// Opaque name suffixes are preserved. Nested error suffixes are translated.
var messagePrefixes = func() []string {
	keys := []string{}
	for key := range english {
		if strings.HasSuffix(key, ": ") || strings.HasSuffix(key, "%w") {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	return keys
}()

func translateMessage(message, language string) string {
	if language != "en" {
		return message
	}
	if translated, ok := english[message]; ok {
		return translated
	}
	for _, key := range messagePrefixes {
		prefix := strings.TrimSuffix(key, "%w")
		if !strings.HasPrefix(message, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(message, prefix)
		switch prefix {
		case "Zeitplan gespeichert: ", "Aufweckversuch: ", "Magic Packet gesendet: ", "Gerät ist online: ", "Kein Erreichbarkeitsnachweis nach Aufwecken: ", "Gerät mit dieser MAC-Adresse bereits vorhanden: ":
			// The entire suffix is a user-supplied name.
		case "Aufwecken fehlgeschlagen: ":
			if index := strings.LastIndex(suffix, " – "); index >= 0 {
				suffix = suffix[:index+len(" – ")] + translateMessage(suffix[index+len(" – "):], language)
			}
		default:
			suffix = translateMessage(suffix, language)
		}
		return strings.TrimSuffix(english[key], "%w") + suffix
	}
	return message // OS error details and external data are retained verbatim.
}

func logMessage(log Log, language string) string {
	if log.MessageKey != "" {
		args := append([]string(nil), log.MessageArgs...)
		if log.MessageKey == "Aufwecken fehlgeschlagen: {0} – {1}" && len(args) > 1 {
			args[1] = translateMessage(args[1], language)
		}
		return formatMessage(log.MessageKey, language, args...)
	}
	return translateMessage(log.Message, language)
}

// Keep the stored state and all user names unchanged. Only message fields in
// the response copy are localized; each request gets its own language.
func localizeData(data any, language string) any {
	if language != "en" {
		return data
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return data
	}
	var result any
	if json.Unmarshal(encoded, &result) != nil {
		return data
	}
	var visit func(any)
	visit = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			for key, field := range node {
				if text, ok := field.(string); ok {
					switch key {
					case "error", "networkError", "message":
						node[key] = translateMessage(text, language)
					}
				}
				if key == "warnings" {
					if items, ok := field.([]any); ok {
						for i, item := range items {
							if text, ok := item.(string); ok {
								items[i] = translateMessage(text, language)
							}
						}
					}
				}
				visit(field)
			}
			if key, ok := node["messageKey"].(string); ok && key != "" {
				args := []string{}
				if values, ok := node["messageArgs"].([]any); ok {
					for _, value := range values {
						args = append(args, fmt.Sprint(value))
					}
				}
				node["message"] = logMessage(Log{MessageKey: key, MessageArgs: args}, language)
			}
			if node["type"] == "LAN-Gerät" {
				node["type"] = english["LAN-Gerät"]
				if ip, ok := node["ip"].(string); ok && node["name"] == fmt.Sprintf("LAN-Gerät (%s)", ip) {
					node["name"] = fmt.Sprintf(english["LAN-Gerät (%s)"], ip)
				}
			}
		case []any:
			for _, item := range node {
				visit(item)
			}
		}
	}
	visit(result)
	return result
}

type languageWriter struct {
	http.ResponseWriter
	language string
}

func forRequest(w http.ResponseWriter, r *http.Request) http.ResponseWriter {
	value := r.Header.Get("Accept-Language")
	// Historical clients with no language preference retain German responses.
	language := "de"
	if value != "" {
		language = normalizeLanguage(strings.Split(strings.Split(value, ",")[0], ";")[0])
	}
	return &languageWriter{w, language}
}

// ErrorText localizes CLI/lifecycle output without changing error identity.
func ErrorText(err error) string { return translateMessage(err.Error(), systemLanguage()) }

// DSM resolves the message template separately for each recipient's language.
// Pass only the unchanged device name, never an already translated sentence.
func notificationArguments(log Log) []string {
	args := []string{"-c", "SYNO.SDS.SynoWake.Application", "-p", "plain", "@administrators", "SynoWake:notification:title"}
	key := "SynoWake:notification:message"
	if len(log.MessageArgs) > 0 {
		switch log.MessageKey {
		case "Magic Packets gesendet: {0}":
			key = "SynoWake:notification:wake_sent"
		case "Aufwecken fehlgeschlagen: {0} – {1}":
			key = "SynoWake:notification:wake_failed"
		}
	}
	args = append(args, key)
	if key != "SynoWake:notification:message" {
		args = append(args, log.MessageArgs[0])
	}
	return args
}
