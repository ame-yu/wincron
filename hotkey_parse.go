package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	modAlt     = 0x0001
	modControl = 0x0002
	modShift   = 0x0004
	modWin     = 0x0008
)

func normalizeHotkeyString(raw string) (normalized string, mod uint32, vk uint32, err error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", 0, 0, nil
	}

	parts := strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case '+', '-', ' ':
			return true
		default:
			return false
		}
	})

	var key string
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		tu := strings.ToUpper(t)
		switch tu {
		case "CTRL", "CONTROL":
			mod |= modControl
			continue
		case "ALT":
			mod |= modAlt
			continue
		case "SHIFT":
			mod |= modShift
			continue
		case "WIN", "WINDOWS", "META":
			mod |= modWin
			continue
		}

		if key != "" {
			return "", 0, 0, errors.New("hotkey must contain exactly one non-modifier key")
		}
		key = tu
	}

	if mod == 0 {
		return "", 0, 0, errors.New("hotkey must include at least one modifier")
	}
	if key == "" {
		return "", 0, 0, errors.New("hotkey key is required")
	}

	keyName, keyVK, err := parseHotkeyKey(key)
	if err != nil {
		return "", 0, 0, err
	}
	vk = keyVK

	mods := make([]string, 0, 4)
	if mod&modControl != 0 {
		mods = append(mods, "Ctrl")
	}
	if mod&modAlt != 0 {
		mods = append(mods, "Alt")
	}
	if mod&modShift != 0 {
		mods = append(mods, "Shift")
	}
	if mod&modWin != 0 {
		mods = append(mods, "Win")
	}
	mods = append(mods, keyName)

	return strings.Join(mods, "+"), mod, vk, nil
}

func parseHotkeyKey(key string) (name string, vk uint32, err error) {
	k := strings.ToUpper(strings.TrimSpace(key))
	if len(k) == 1 {
		r := k[0]
		if r >= 'A' && r <= 'Z' {
			return k, uint32(r), nil
		}
		if r >= '0' && r <= '9' {
			return k, uint32(r), nil
		}
	}

	if strings.HasPrefix(k, "F") && len(k) >= 2 {
		n, err := strconv.Atoi(k[1:])
		if err == nil && n >= 1 && n <= 24 {
			return fmt.Sprintf("F%d", n), 0x70 + uint32(n-1), nil
		}
	}

	switch k {
	case "SPACE":
		return "Space", 0x20, nil
	case "ENTER", "RETURN":
		return "Enter", 0x0D, nil
	case "ESC", "ESCAPE":
		return "Esc", 0x1B, nil
	case "TAB":
		return "Tab", 0x09, nil
	case "BACKSPACE", "BACK":
		return "Backspace", 0x08, nil
	case "DELETE", "DEL":
		return "Del", 0x2E, nil
	case "INSERT", "INS":
		return "Ins", 0x2D, nil
	case "HOME":
		return "Home", 0x24, nil
	case "END":
		return "End", 0x23, nil
	case "PAGEUP", "PGUP", "PRIOR":
		return "PgUp", 0x21, nil
	case "PAGEDOWN", "PGDN", "NEXT":
		return "PgDn", 0x22, nil
	case "UP":
		return "Up", 0x26, nil
	case "DOWN":
		return "Down", 0x28, nil
	case "LEFT":
		return "Left", 0x25, nil
	case "RIGHT":
		return "Right", 0x27, nil
	}

	return "", 0, fmt.Errorf("unsupported hotkey key: %s", key)
}
