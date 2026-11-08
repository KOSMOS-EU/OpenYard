package metadata

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// FolderPath builds the storage folder for an imported document from the
// namespace's folder templates (NamespaceConfig.Folders, FolderDefault).
//
// Templates are path strings with placeholders in braces:
//
//	Anordnungen/{HH_JAHR}/{KZ_DKS}/{AO_NR|block:1000}
//	Veranlagung/{HH_JAHR}/{BART}
//	Ablage/{doc.type}/{now|date:2006}/{now|date:01}
//
// A placeholder names a client metadata key of the DocIndex (case-insensitive),
// "doc.type" (the ConfigName) or "now" (import time). Modifiers follow after "|":
//
//	block:N     number range the value falls into, N per block: 7151 → 7001-8000
//	pad:N       left-pad with zeros to N digits: 42 → 0042
//	left:N      first N characters, right:N last N characters
//	upper, lower
//	date:LAYOUT re-format a date (client format of the namespace, ISO, or now) with a Go layout
//	default:X   value if the placeholder is empty so far
//
// A placeholder that stays empty makes the template fail; the namespace's
// folder_default is tried next. Returns "" if no template applies: the
// document then lands where the uploader would put it anyway (space root).
// Slashes inside a value are replaced by U+2215 so a value never adds levels.
func (c *Config) FolderPath(login, docType string, index map[string]string, now time.Time) string {
	ns := c.NamespaceForUser(login)
	if template, ok := ns.Folders[docType]; ok && template != "" {
		if folder, err := expandFolderTemplate(template, docType, index, now, ns.Formats.Date); err == nil {
			return folder
		}
	}
	if ns.FolderDefault != "" {
		if folder, err := expandFolderTemplate(ns.FolderDefault, docType, index, now, ns.Formats.Date); err == nil {
			return folder
		}
	}
	return ""
}

// ExpandTemplate renders a folder or name template for the user's namespace
// with the DocIndex values plus extra placeholders (e.g. lookup.code,
// lookup.name); same syntax and rules as FolderPath.
func (c *Config) ExpandTemplate(login, template, docType string, index, extra map[string]string, now time.Time) (string, error) {
	ns := c.NamespaceForUser(login)
	return expandFolderTemplateWith(template, docType, index, extra, now, ns.Formats.Date)
}

func expandFolderTemplate(template, docType string, index map[string]string, now time.Time, clientDateLayout string) (string, error) {
	return expandFolderTemplateWith(template, docType, index, nil, now, clientDateLayout)
}

func expandFolderTemplateWith(template, docType string, index, extra map[string]string, now time.Time, clientDateLayout string) (string, error) {
	lookup := make(map[string]string, len(index)+len(extra)+1)
	for key, value := range index {
		lookup[strings.ToLower(key)] = value
	}
	lookup["doc.type"] = docType
	for key, value := range extra {
		lookup[strings.ToLower(key)] = value
	}

	var out strings.Builder
	rest := template
	for {
		start := strings.IndexByte(rest, '{')
		if start < 0 {
			out.WriteString(rest)
			break
		}
		end := strings.IndexByte(rest[start:], '}')
		if end < 0 {
			return "", fmt.Errorf("folder template %q: unclosed placeholder", template)
		}
		out.WriteString(rest[:start])
		placeholder := rest[start+1 : start+end]
		rest = rest[start+end+1:]

		value, err := expandPlaceholder(placeholder, lookup, now, clientDateLayout)
		if err != nil {
			return "", fmt.Errorf("folder template %q: %w", template, err)
		}
		out.WriteString(strings.ReplaceAll(value, "/", "∕"))
	}

	var segments []string
	for _, segment := range strings.Split(out.String(), "/") {
		segment = strings.TrimSpace(segment)
		if segment == "" || segment == "." || segment == ".." {
			continue
		}
		segments = append(segments, segment)
	}
	if len(segments) == 0 {
		return "", fmt.Errorf("folder template %q: empty path", template)
	}
	return strings.Join(segments, "/"), nil
}

func expandPlaceholder(placeholder string, lookup map[string]string, now time.Time, clientDateLayout string) (string, error) {
	parts := strings.Split(placeholder, "|")
	name := strings.TrimSpace(parts[0])
	var value string
	if strings.EqualFold(name, "now") {
		value = now.Format(isoDateLayout)
	} else {
		value = strings.TrimSpace(lookup[strings.ToLower(name)])
	}

	for _, modifier := range parts[1:] {
		modName, arg, _ := strings.Cut(strings.TrimSpace(modifier), ":")
		switch strings.ToLower(modName) {
		case "default":
			if value == "" {
				value = arg
			}
		case "upper":
			value = strings.ToUpper(value)
		case "lower":
			value = strings.ToLower(value)
		case "left":
			n, err := strconv.Atoi(arg)
			if err != nil {
				return "", fmt.Errorf("left: %w", err)
			}
			if runes := []rune(value); len(runes) > n {
				value = string(runes[:n])
			}
		case "right":
			n, err := strconv.Atoi(arg)
			if err != nil {
				return "", fmt.Errorf("right: %w", err)
			}
			if runes := []rune(value); len(runes) > n {
				value = string(runes[len(runes)-n:])
			}
		case "pad":
			n, err := strconv.Atoi(arg)
			if err != nil {
				return "", fmt.Errorf("pad: %w", err)
			}
			if value != "" {
				value = fmt.Sprintf("%0*s", n, value)
				value = strings.ReplaceAll(value, " ", "0")
			}
		case "block":
			if value == "" {
				continue
			}
			size, err := strconv.Atoi(arg)
			if err != nil || size <= 0 {
				return "", fmt.Errorf("block: size %q", arg)
			}
			number, err := strconv.Atoi(strings.TrimLeft(value, "0"))
			if err != nil {
				return "", fmt.Errorf("block: %q is not a number", value)
			}
			if number <= 0 {
				number = 1
			}
			low := ((number-1)/size)*size + 1
			value = fmt.Sprintf("%d-%d", low, low+size-1)
		case "date":
			if value == "" {
				continue
			}
			parsed, err := parseClientOrISODate(value, clientDateLayout)
			if err != nil {
				return "", fmt.Errorf("date: %w", err)
			}
			if arg == "" {
				arg = isoDateLayout
			}
			value = parsed.Format(arg)
		default:
			return "", fmt.Errorf("unknown modifier %q", modName)
		}
	}
	if value == "" {
		return "", fmt.Errorf("placeholder %q is empty", name)
	}
	return value, nil
}

func parseClientOrISODate(value, clientDateLayout string) (time.Time, error) {
	layouts := []string{isoDateLayout, time.RFC3339, "2006-01-02T15:04:05"}
	if clientDateLayout != "" {
		layouts = append([]string{clientDateLayout}, layouts...)
	}
	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}
