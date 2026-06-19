package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const backupSuffix = ".ss220patch.bak"

var launcherDirNames = []string{
	"",
	"SS14.Launcher_Linux",
	"SS14.Launcher_Windows",
	"SS14.Launcher_macOS",
	"SS14.Launcher_MacOS",
	"SS14.Launcher_OSX",
	"SS14.Launcher_Darwin",
	"SS14.Launcher.app",
}

var launcherBinDirs = []string{
	"bin_x64",
	"bin_arm64",
	"bin",
	filepath.Join("Contents", "Resources", "bin_x64"),
	filepath.Join("Contents", "Resources", "bin_arm64"),
	filepath.Join("Contents", "MacOS"),
}

var urlPairs = []struct {
	Old string
	New string
}{
	{
		Old: "https://robust-builds.cdn.spacestation14.com/",
		New: "https://robust.ss14.ss220.club/builds-cdn-v1/",
	},
	{
		Old: "https://robust-builds.fallback.cdn.spacestation14.com/",
		New: "https://robust-fb.ss14.ss220.club/builds-cdn-fb-v001x/",
	},
	{
		Old: "https://launcher-data.cdn.spacestation14.com/",
		New: "https://launcher.ss14.ss220.club/data-cdn-v1/",
	},
	{
		Old: "https://launcher-data.fallback.cdn.spacestation14.com/",
		New: "https://launcher-fb.ss14.ss220.club/data-cdn-fb-v001x/",
	},
}

func main() {
	if err := validateURLPairs(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	a := app.NewWithID("club.ss220.ss14-url-patcher")
	w := a.NewWindow("SS14 CDN URL patcher")
	w.Resize(fyne.NewSize(760, 360))

	pathEntry := widget.NewEntry()
	pathEntry.SetText(defaultSteamSS14Path())
	pathEntry.SetPlaceHolder("Папка игры Space Station 14")

	status := widget.NewMultiLineEntry()
	status.SetText("Выберите папку игры Space Station 14 и нажмите \"Запатчить\".\nОткат работает из backup-файлов рядом с DLL.")
	status.Disable()

	chooseButton := widget.NewButton("Выбрать папку", func() {
		fd := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if uri == nil {
				return
			}
			pathEntry.SetText(uri.Path())
		}, w)
		fd.Show()
	})

	patchButton := widget.NewButton("Запатчить", func() {
		root := strings.TrimSpace(pathEntry.Text)
		msg, err := patchGame(root)
		if err != nil {
			status.SetText(msg + "\n\nОшибка: " + err.Error())
			dialog.ShowError(err, w)
			return
		}
		status.SetText(msg)
	})

	rollbackButton := widget.NewButton("Откатить", func() {
		root := strings.TrimSpace(pathEntry.Text)
		msg, err := rollbackGame(root)
		if err != nil {
			status.SetText(msg + "\n\nОшибка: " + err.Error())
			dialog.ShowError(err, w)
			return
		}
		status.SetText(msg)
	})

	pathRow := container.NewBorder(nil, nil, nil, chooseButton, pathEntry)
	buttons := container.NewHBox(patchButton, rollbackButton)
	content := container.NewBorder(
		container.NewVBox(
			widget.NewLabel("Папка игры Space Station 14:"),
			pathRow,
			buttons,
			widget.NewSeparator(),
		),
		nil,
		nil,
		nil,
		status,
	)

	w.SetContent(content)
	w.ShowAndRun()
}

func validateURLPairs() error {
	for _, p := range urlPairs {
		if len(p.Old) != len(p.New) {
			return fmt.Errorf("URL length mismatch: old=%d new=%d\nold=%s\nnew=%s", len(p.Old), len(p.New), p.Old, p.New)
		}
	}
	return nil
}

func defaultSteamSS14Path() string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "windows":
		candidates := []string{
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "Steam", "steamapps", "common", "Space Station 14"),
			filepath.Join(os.Getenv("ProgramFiles"), "Steam", "steamapps", "common", "Space Station 14"),
		}
		for _, p := range candidates {
			if p != "" && dirExists(p) {
				return p
			}
		}
		if os.Getenv("ProgramFiles(x86)") != "" {
			return filepath.Join(os.Getenv("ProgramFiles(x86)"), "Steam", "steamapps", "common", "Space Station 14")
		}
		return `C:\Program Files (x86)\Steam\steamapps\common\Space Station 14`

	case "darwin":
		p := filepath.Join(home, "Library", "Application Support", "Steam", "steamapps", "common", "Space Station 14")
		return p

	default:
		candidates := []string{
			filepath.Join(home, ".local", "share", "Steam", "steamapps", "common", "Space Station 14"),
			filepath.Join(home, ".steam", "steam", "steamapps", "common", "Space Station 14"),
			filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam", "steamapps", "common", "Space Station 14"),
		}
		for _, p := range candidates {
			if dirExists(p) {
				return p
			}
		}
		return candidates[0]
	}
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func patchGame(root string) (string, error) {
	if root == "" {
		return "", errors.New("не указан путь к папке игры или лаунчера")
	}

	targets := findLauncherDLLs(root)

	var out []string
	out = append(out, "Патчинг URL в папке:")
	out = append(out, root)
	out = append(out, "")

	if len(targets) == 0 {
		out = append(out, "Целевые DLL не найдены.")
		out = append(out, "")
		out = append(out, "Можно выбрать один из вариантов:")
		out = append(out, "- саму папку лаунчера, где лежат bin_x64/bin_arm64")
		out = append(out, "- папку игры Steam, внутри которой лежит SS14.Launcher_Linux или SS14.Launcher_Windows")
		out = append(out, "- конкретную распакованную папку SS14.Launcher_Linux / SS14.Launcher_Windows")
		return strings.Join(out, "\n"), errors.New("SS14.Launcher.dll не найден")
	}

	patched := 0
	for _, target := range targets {
		result, err := patchFile(target)
		out = append(out, result)
		if err != nil {
			return strings.Join(out, "\n"), err
		}
		if strings.HasPrefix(result, "PATCHED") {
			patched++
		}
	}

	out = append(out, "")
	out = append(out, fmt.Sprintf("Итог: patched=%d, targets=%d", patched, len(targets)))
	return strings.Join(out, "\n"), nil
}

func rollbackGame(root string) (string, error) {
	if root == "" {
		return "", errors.New("не указан путь к папке игры или лаунчера")
	}

	targets := findLauncherDLLs(root)

	var out []string
	out = append(out, "Откат в папке:")
	out = append(out, root)
	out = append(out, "")

	if len(targets) == 0 {
		out = append(out, "Целевые DLL не найдены.")
		return strings.Join(out, "\n"), errors.New("SS14.Launcher.dll не найден")
	}

	restored := 0
	backupSeen := 0

	for _, path := range targets {
		rel := displayPath(path)
		backup := path + backupSuffix

		if _, err := os.Stat(backup); err != nil {
			out = append(out, "SKIP no backup: "+rel)
			continue
		}
		backupSeen++

		data, err := os.ReadFile(backup)
		if err != nil {
			out = append(out, "ERROR read backup: "+backup)
			return strings.Join(out, "\n"), err
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			out = append(out, "ERROR restore: "+path)
			return strings.Join(out, "\n"), err
		}

		out = append(out, "RESTORED: "+rel)
		restored++
	}

	if backupSeen == 0 {
		out = append(out, "")
		return strings.Join(out, "\n"), errors.New("backup-файлы не найдены; откат невозможен")
	}

	out = append(out, "")
	out = append(out, fmt.Sprintf("Итог: restored=%d", restored))
	return strings.Join(out, "\n"), nil
}

func findLauncherDLLs(root string) []string {
	root = filepath.Clean(root)
	seen := make(map[string]struct{})
	var targets []string

	add := func(path string) {
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			return
		}
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			return
		}
		seen[path] = struct{}{}
		targets = append(targets, path)
	}

	for _, dirName := range launcherDirNames {
		for _, binDir := range launcherBinDirs {
			if dirName == "" {
				add(filepath.Join(root, binDir, "SS14.Launcher.dll"))
				continue
			}
			add(filepath.Join(root, dirName, binDir, "SS14.Launcher.dll"))
		}
	}

	// Fallback for layout changes: shallow recursive search only.
	// This covers direct launcher folders, Steam folders and macOS .app layouts.
	maxDepth := pathDepth(root) + 8
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "dotnet_x64" || name == "dotnet_arm64" {
				return filepath.SkipDir
			}
			if pathDepth(path) > maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "SS14.Launcher.dll" {
			add(path)
		}
		return nil
	})

	return targets
}

func pathDepth(path string) int {
	path = filepath.Clean(path)
	if path == "." || path == string(os.PathSeparator) {
		return 0
	}
	return len(strings.Split(path, string(os.PathSeparator)))
}

func displayPath(path string) string {
	parts := []string{}
	clean := filepath.Clean(path)
	for i := 0; i < 3; i++ {
		base := filepath.Base(clean)
		if base == "." || base == string(os.PathSeparator) || base == "" {
			break
		}
		parts = append([]string{base}, parts...)
		clean = filepath.Dir(clean)
	}
	return filepath.Join(parts...)
}

func patchFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "ERROR read: " + path, err
	}

	orig := data
	oldHits := 0
	newHits := 0

	for _, p := range urlPairs {
		oldUTF8 := []byte(p.Old)
		newUTF8 := []byte(p.New)
		oldUTF16 := utf16LE(p.Old)
		newUTF16 := utf16LE(p.New)

		oldHits += bytes.Count(data, oldUTF8)
		oldHits += bytes.Count(data, oldUTF16)
		newHits += bytes.Count(data, newUTF8)
		newHits += bytes.Count(data, newUTF16)

		data = bytes.ReplaceAll(data, oldUTF8, newUTF8)
		data = bytes.ReplaceAll(data, oldUTF16, newUTF16)
	}

	rel := displayPath(path)

	if bytes.Equal(data, orig) {
		if oldHits == 0 && newHits > 0 {
			return "ALREADY PATCHED: " + rel, nil
		}
		return fmt.Sprintf("NO URL FOUND: %s", rel), nil
	}

	backup := path + backupSuffix
	if _, err := os.Stat(backup); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(backup, orig, 0644); err != nil {
			return "ERROR backup: " + backup, err
		}
	}

	mode := os.FileMode(0644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		return "ERROR write: " + path, err
	}

	return fmt.Sprintf("PATCHED: %s, replacements=%d", rel, oldHits), nil
}

func utf16LE(s string) []byte {
	runes := utf16.Encode([]rune(s))
	buf := make([]byte, len(runes)*2)
	for i, r := range runes {
		binary.LittleEndian.PutUint16(buf[i*2:], r)
	}
	return buf
}
