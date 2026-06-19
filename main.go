package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

const backupSuffix = ".ss220patch.bak"

type urlPair struct {
	Old string
	New string
}

type logLevel int

const (
	levelTitle logLevel = iota
	levelInfo
	levelOK
	levelWarn
	levelError
	levelSkip
	levelSummary
)

type logEntry struct {
	Level logLevel
	Text  string
}

type colorLog struct {
	box    *fyne.Container
	scroll *container.Scroll
}

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

var urlPairs = []urlPair{
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
	w.Resize(fyne.NewSize(860, 520))

	pathEntry := widget.NewEntry()
	pathEntry.SetText(defaultSteamSS14Path())
	pathEntry.SetPlaceHolder("Папка игры Space Station 14 или папка SS14.Launcher_*")

	logView := newColorLog()
	logView.set([]logEntry{
		{Level: levelTitle, Text: "SS14 CDN URL patcher"},
		{Level: levelInfo, Text: "Выберите папку игры Space Station 14 или распакованную папку лаунчера."},
		{Level: levelInfo, Text: "Кнопка выбора сначала пробует системный Explorer/Finder/zenity/kdialog/yad, затем Fyne fallback."},
		{Level: levelWarn, Text: "Backup не перезаписывается. Откат удаляет backup после успешного восстановления."},
	})

	chooseButton := widget.NewButton("Выбрать папку", func() {
		chooseLauncherDirNativeFirst(w, pathEntry.Text, func(path string) {
			pathEntry.SetText(path)
		})
	})

	checkButton := widget.NewButton("Проверить", func() {
		root := strings.TrimSpace(pathEntry.Text)
		entries, err := checkGame(root)
		logView.set(entries)
		if err != nil {
			dialog.ShowError(err, w)
		}
	})

	patchButton := widget.NewButton("Запатчить", func() {
		root := strings.TrimSpace(pathEntry.Text)
		entries, err := patchGame(root)
		logView.set(entries)
		if err != nil {
			dialog.ShowError(err, w)
		}
	})

	rollbackButton := widget.NewButton("Откатить", func() {
		root := strings.TrimSpace(pathEntry.Text)
		entries, err := rollbackGame(root)
		logView.set(entries)
		if err != nil {
			dialog.ShowError(err, w)
		}
	})

	pathRow := container.NewBorder(nil, nil, nil, chooseButton, pathEntry)
	buttons := container.NewHBox(checkButton, patchButton, rollbackButton)
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
		logView.scroll,
	)

	w.SetContent(content)
	w.ShowAndRun()
}

func newColorLog() *colorLog {
	box := container.NewVBox()
	scroll := container.NewVScroll(box)
	return &colorLog{box: box, scroll: scroll}
}

func (l *colorLog) set(entries []logEntry) {
	l.box.Objects = nil

	for _, entry := range entries {
		line := canvas.NewText(formatLogLine(entry), logColor(entry.Level))
		line.TextSize = 13
		line.TextStyle = fyne.TextStyle{Monospace: true}
		l.box.Add(line)
	}

	l.box.Refresh()
	l.scroll.ScrollToTop()
}

func formatLogLine(entry logEntry) string {
	prefix := "[INFO] "
	switch entry.Level {
	case levelTitle:
		prefix = "[----] "
	case levelOK:
		prefix = "[ OK ] "
	case levelWarn:
		prefix = "[WARN] "
	case levelError:
		prefix = "[ERR ] "
	case levelSkip:
		prefix = "[SKIP] "
	case levelSummary:
		prefix = "[SUM ] "
	}
	return prefix + entry.Text
}

func logColor(level logLevel) color.Color {
	switch level {
	case levelTitle:
		return color.RGBA{R: 21, G: 101, B: 192, A: 255}
	case levelOK:
		return color.RGBA{R: 46, G: 125, B: 50, A: 255}
	case levelWarn:
		return color.RGBA{R: 191, G: 111, B: 0, A: 255}
	case levelError:
		return color.RGBA{R: 198, G: 40, B: 40, A: 255}
	case levelSkip:
		return color.RGBA{R: 96, G: 125, B: 139, A: 255}
	case levelSummary:
		return color.RGBA{R: 74, G: 20, B: 140, A: 255}
	default:
		return color.RGBA{R: 55, G: 71, B: 79, A: 255}
	}
}

func validateURLPairs() error {
	for _, p := range urlPairs {
		if len(p.Old) != len(p.New) {
			return fmt.Errorf("URL length mismatch: old=%d new=%d\nold=%s\nnew=%s", len(p.Old), len(p.New), p.Old, p.New)
		}
	}
	return nil
}

func chooseLauncherDirNativeFirst(parent fyne.Window, current string, setPath func(string)) {
	start := bestExistingStartDir(current)

	if dir, err := chooseDirNative(start); err == nil && dir != "" {
		setPath(dir)
		return
	}

	chooseDirFyne(parent, start, setPath)
}

func chooseDirNative(start string) (string, error) {
	switch runtime.GOOS {
	case "windows":
		return chooseDirWindows(start)
	case "darwin":
		return chooseDirDarwin(start)
	default:
		return chooseDirLinux(start)
	}
}

func chooseDirWindows(start string) (string, error) {
	script := `
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()
Add-Type -AssemblyName System.Windows.Forms
$dialog = New-Object System.Windows.Forms.FolderBrowserDialog
$dialog.Description = 'Выберите папку Space Station 14 или SS14 Launcher'
$dialog.ShowNewFolderButton = $false
$start = [Environment]::GetEnvironmentVariable('SS14_START_DIR')
if (![string]::IsNullOrWhiteSpace($start) -and (Test-Path -LiteralPath $start)) {
    $dialog.SelectedPath = $start
}
$result = $dialog.ShowDialog()
if ($result -eq [System.Windows.Forms.DialogResult]::OK) {
    Write-Output $dialog.SelectedPath
    exit 0
}
exit 1
`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-STA", "-Command", script)
	cmd.Env = append(os.Environ(), "SS14_START_DIR="+start)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return cleanCommandPath(string(out))
}

func chooseDirDarwin(start string) (string, error) {
	script := `
set startPath to system attribute "SS14_START_DIR"
if startPath is not "" then
    set chosenFolder to choose folder with prompt "Выберите папку Space Station 14 или SS14 Launcher" default location (POSIX file startPath)
else
    set chosenFolder to choose folder with prompt "Выберите папку Space Station 14 или SS14 Launcher"
end if
POSIX path of chosenFolder
`
	cmd := exec.Command("osascript", "-e", script)
	cmd.Env = append(os.Environ(), "SS14_START_DIR="+start)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return cleanCommandPath(string(out))
}

func chooseDirLinux(start string) (string, error) {
	if path, err := chooseDirLinuxZenity(start); err == nil && path != "" {
		return path, nil
	}
	if path, err := chooseDirLinuxKDialog(start); err == nil && path != "" {
		return path, nil
	}
	if path, err := chooseDirLinuxYad(start); err == nil && path != "" {
		return path, nil
	}
	return "", errors.New("no native linux folder picker found")
}

func chooseDirLinuxZenity(start string) (string, error) {
	if _, err := exec.LookPath("zenity"); err != nil {
		return "", err
	}

	args := []string{
		"--file-selection",
		"--directory",
		"--title=Выберите папку Space Station 14 или SS14 Launcher",
	}
	if start != "" {
		args = append(args, "--filename="+ensureTrailingSeparator(start))
	}

	out, err := exec.Command("zenity", args...).Output()
	if err != nil {
		return "", err
	}
	return cleanCommandPath(string(out))
}

func chooseDirLinuxKDialog(start string) (string, error) {
	if _, err := exec.LookPath("kdialog"); err != nil {
		return "", err
	}

	args := []string{"--getexistingdirectory"}
	if start != "" {
		args = append(args, start)
	}
	args = append(args, "Выберите папку Space Station 14 или SS14 Launcher")

	out, err := exec.Command("kdialog", args...).Output()
	if err != nil {
		return "", err
	}
	return cleanCommandPath(string(out))
}

func chooseDirLinuxYad(start string) (string, error) {
	if _, err := exec.LookPath("yad"); err != nil {
		return "", err
	}

	args := []string{
		"--file",
		"--directory",
		"--title=Выберите папку Space Station 14 или SS14 Launcher",
	}
	if start != "" {
		args = append(args, "--filename="+ensureTrailingSeparator(start))
	}

	out, err := exec.Command("yad", args...).Output()
	if err != nil {
		return "", err
	}
	return cleanCommandPath(string(out))
}

func chooseDirFyne(parent fyne.Window, start string, setPath func(string)) {
	d := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}

		path := uri.Path()
		if path == "" {
			return
		}

		setPath(filepath.Clean(path))
	}, parent)

	if start != "" {
		if lister, err := storage.ListerForURI(storage.NewFileURI(start)); err == nil {
			d.SetLocation(lister)
		}
	}

	d.Show()
}

func cleanCommandPath(out string) (string, error) {
	path := strings.TrimSpace(out)
	path = strings.Trim(path, "\x00\r\n\t ")
	if path == "" {
		return "", errors.New("empty selection")
	}
	return filepath.Clean(path), nil
}

func ensureTrailingSeparator(path string) string {
	if path == "" {
		return path
	}
	if strings.HasSuffix(path, string(os.PathSeparator)) {
		return path
	}
	return path + string(os.PathSeparator)
}

func bestExistingStartDir(current string) string {
	candidates := make([]string, 0, 20)

	if current != "" {
		candidates = append(candidates, current)
	}

	candidates = append(candidates, defaultLauncherRoots()...)

	home, _ := os.UserHomeDir()
	if home != "" {
		candidates = append(candidates,
			filepath.Join(home, "Desktop"),
			filepath.Join(home, "Downloads"),
			home,
		)
	}

	for _, p := range candidates {
		if p == "" {
			continue
		}

		p = expandHome(p)

		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return filepath.Clean(p)
		}

		parent := filepath.Dir(p)
		if parent != "." && parent != p {
			if st, err := os.Stat(parent); err == nil && st.IsDir() {
				return filepath.Clean(parent)
			}
		}
	}

	return ""
}

func defaultLauncherRoots() []string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "windows":
		return []string{
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "Steam", "steamapps", "common", "Space Station 14"),
			filepath.Join(os.Getenv("ProgramFiles"), "Steam", "steamapps", "common", "Space Station 14"),
			filepath.Join(home, "AppData", "Local", "Steam", "steamapps", "common", "Space Station 14"),
		}
	case "darwin":
		return []string{
			filepath.Join(home, "Library", "Application Support", "Steam", "steamapps", "common", "Space Station 14"),
			filepath.Join(home, "Desktop", "games", "SS14.Launcher_macOS"),
			filepath.Join(home, "Desktop", "games", "SS14.Launcher.app"),
		}
	default:
		return []string{
			filepath.Join(home, ".steam", "steam", "steamapps", "common", "Space Station 14"),
			filepath.Join(home, ".local", "share", "Steam", "steamapps", "common", "Space Station 14"),
			filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam", "steamapps", "common", "Space Station 14"),
			filepath.Join(home, "Desktop", "games", "SS14.Launcher_Linux"),
			filepath.Join(home, "Games", "SS14.Launcher_Linux"),
		}
	}
}

func defaultSteamSS14Path() string {
	for _, p := range defaultLauncherRoots() {
		if p != "" && dirExists(p) {
			return p
		}
	}

	roots := defaultLauncherRoots()
	if len(roots) > 0 && roots[0] != "" {
		return roots[0]
	}

	home, _ := os.UserHomeDir()
	return home
}

func expandHome(path string) string {
	if path == "" {
		return path
	}

	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}

	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, _ := os.UserHomeDir()
		if home == "" {
			return path
		}
		return filepath.Join(home, path[2:])
	}

	return path
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func checkGame(root string) ([]logEntry, error) {
	if root == "" {
		return []logEntry{{Level: levelError, Text: "Не указан путь к папке игры или лаунчера."}}, errors.New("не указан путь к папке игры или лаунчера")
	}

	targets := findLauncherDLLs(root)
	entries := []logEntry{
		{Level: levelTitle, Text: "Проверка папки"},
		{Level: levelInfo, Text: root},
		{Level: levelInfo, Text: fmt.Sprintf("Найдено DLL: %d", len(targets))},
	}

	if len(targets) == 0 {
		entries = append(entries,
			logEntry{Level: levelError, Text: "SS14.Launcher.dll не найден."},
			logEntry{Level: levelInfo, Text: "Можно выбрать Steam-папку игры, папку SS14.Launcher_Linux/Windows/macOS или родительскую папку."},
		)
		return entries, errors.New("SS14.Launcher.dll не найден")
	}

	backups := 0
	patched := 0
	original := 0
	unknown := 0

	for _, path := range targets {
		state, err := inspectFileState(path)
		rel := displayPath(path)
		if err != nil {
			entries = append(entries, logEntry{Level: levelError, Text: fmt.Sprintf("%s: %v", rel, err)})
			unknown++
			continue
		}

		if state.BackupExists {
			backups++
		}
		if state.CurrentOldHits > 0 {
			original++
		}
		if state.CurrentNewHits > 0 && state.CurrentOldHits == 0 {
			patched++
		}
		if state.CurrentOldHits == 0 && state.CurrentNewHits == 0 {
			unknown++
		}

		entries = append(entries, formatStateLog(rel, state)...)
	}

	entries = append(entries,
		logEntry{Level: levelSummary, Text: fmt.Sprintf("Итог: dll=%d, backups=%d, original-like=%d, patched-like=%d, unknown=%d", len(targets), backups, original, patched, unknown)},
	)

	return entries, nil
}

func patchGame(root string) ([]logEntry, error) {
	if root == "" {
		return []logEntry{{Level: levelError, Text: "Не указан путь к папке игры или лаунчера."}}, errors.New("не указан путь к папке игры или лаунчера")
	}

	targets := findLauncherDLLs(root)
	entries := []logEntry{
		{Level: levelTitle, Text: "Патчинг URL"},
		{Level: levelInfo, Text: root},
	}

	if len(targets) == 0 {
		entries = append(entries,
			logEntry{Level: levelError, Text: "SS14.Launcher.dll не найден."},
			logEntry{Level: levelInfo, Text: "Можно выбрать Steam-папку игры, папку SS14.Launcher_Linux/Windows/macOS или родительскую папку."},
		)
		return entries, errors.New("SS14.Launcher.dll не найден")
	}

	patched := 0
	skipped := 0

	for _, target := range targets {
		result, err := patchFile(target)
		entries = append(entries, result.Entry)
		if err != nil {
			return entries, err
		}
		if result.Patched {
			patched++
		} else {
			skipped++
		}
	}

	entries = append(entries, logEntry{Level: levelSummary, Text: fmt.Sprintf("Итог: patched=%d, skipped=%d, targets=%d", patched, skipped, len(targets))})
	return entries, nil
}

func rollbackGame(root string) ([]logEntry, error) {
	if root == "" {
		return []logEntry{{Level: levelError, Text: "Не указан путь к папке игры или лаунчера."}}, errors.New("не указан путь к папке игры или лаунчера")
	}

	targets := findLauncherDLLs(root)
	entries := []logEntry{
		{Level: levelTitle, Text: "Откат из backup"},
		{Level: levelInfo, Text: root},
	}

	if len(targets) == 0 {
		entries = append(entries,
			logEntry{Level: levelError, Text: "SS14.Launcher.dll не найден."},
		)
		return entries, errors.New("SS14.Launcher.dll не найден")
	}

	restored := 0
	backupSeen := 0
	skipped := 0

	for _, path := range targets {
		result, err := rollbackFile(path)
		entries = append(entries, result.Entry)
		if err != nil {
			return entries, err
		}
		if result.BackupSeen {
			backupSeen++
		}
		if result.Restored {
			restored++
		} else {
			skipped++
		}
	}

	if backupSeen == 0 {
		entries = append(entries,
			logEntry{Level: levelError, Text: "Backup-файлы не найдены. Откат невозможен."},
		)
		return entries, errors.New("backup-файлы не найдены; откат невозможен")
	}

	entries = append(entries, logEntry{Level: levelSummary, Text: fmt.Sprintf("Итог: restored=%d, skipped=%d, backups_seen=%d", restored, skipped, backupSeen)})
	return entries, nil
}

type fileState struct {
	CurrentOldHits int
	CurrentNewHits int
	BackupExists   bool
	BackupOldHits  int
	BackupNewHits  int
	BackupSame     bool
}

type patchResult struct {
	Entry   logEntry
	Patched bool
}

type rollbackResult struct {
	Entry      logEntry
	Restored   bool
	BackupSeen bool
}

func inspectFileState(path string) (fileState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fileState{}, err
	}

	state := fileState{}
	state.CurrentOldHits, state.CurrentNewHits = countURLHits(data)

	backup := path + backupSuffix
	backupData, err := os.ReadFile(backup)
	if err == nil {
		state.BackupExists = true
		state.BackupOldHits, state.BackupNewHits = countURLHits(backupData)
		state.BackupSame = bytes.Equal(data, backupData)
		return state, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}

	return state, err
}

func formatStateLog(rel string, state fileState) []logEntry {
	entries := make([]logEntry, 0, 2)

	switch {
	case state.CurrentOldHits > 0 && state.CurrentNewHits == 0:
		entries = append(entries, logEntry{Level: levelInfo, Text: fmt.Sprintf("%s: original-like, old_urls=%d", rel, state.CurrentOldHits)})
	case state.CurrentOldHits == 0 && state.CurrentNewHits > 0:
		entries = append(entries, logEntry{Level: levelOK, Text: fmt.Sprintf("%s: patched-like, new_urls=%d", rel, state.CurrentNewHits)})
	case state.CurrentOldHits > 0 && state.CurrentNewHits > 0:
		entries = append(entries, logEntry{Level: levelWarn, Text: fmt.Sprintf("%s: partial/mixed state, old_urls=%d, new_urls=%d", rel, state.CurrentOldHits, state.CurrentNewHits)})
	default:
		entries = append(entries, logEntry{Level: levelWarn, Text: fmt.Sprintf("%s: known URLs not found", rel)})
	}

	if state.BackupExists {
		if state.BackupOldHits > 0 && state.BackupNewHits == 0 {
			entries = append(entries, logEntry{Level: levelOK, Text: fmt.Sprintf("%s: backup exists and looks original, old_urls=%d", rel, state.BackupOldHits)})
		} else {
			entries = append(entries, logEntry{Level: levelWarn, Text: fmt.Sprintf("%s: backup exists but is suspicious, old_urls=%d, new_urls=%d", rel, state.BackupOldHits, state.BackupNewHits)})
		}
	} else {
		entries = append(entries, logEntry{Level: levelSkip, Text: fmt.Sprintf("%s: backup missing", rel)})
	}

	return entries
}

func patchFile(path string) (patchResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return patchResult{Entry: logEntry{Level: levelError, Text: "ERROR read: " + path}}, err
	}

	orig := data
	oldHits, newHits := countURLHits(data)
	rel := displayPath(path)
	backup := path + backupSuffix

	backupData, backupErr := os.ReadFile(backup)
	backupExists := backupErr == nil
	if backupErr != nil && !errors.Is(backupErr, os.ErrNotExist) {
		return patchResult{Entry: logEntry{Level: levelError, Text: "ERROR read backup: " + backup}}, backupErr
	}

	if oldHits == 0 && newHits > 0 {
		if backupExists {
			return patchResult{Entry: logEntry{Level: levelSkip, Text: "ALREADY PATCHED: " + rel + " ; backup preserved"}}, nil
		}
		return patchResult{Entry: logEntry{Level: levelWarn, Text: "ALREADY PATCHED WITHOUT BACKUP: " + rel + " ; rollback unavailable"}}, nil
	}

	if oldHits == 0 && newHits == 0 {
		return patchResult{Entry: logEntry{Level: levelWarn, Text: "NO KNOWN URL FOUND: " + rel}}, nil
	}

	if backupExists {
		backupOldHits, backupNewHits := countURLHits(backupData)
		if backupOldHits == 0 || backupNewHits > 0 {
			return patchResult{Entry: logEntry{Level: levelError, Text: "REFUSE PATCH: " + rel + " ; backup exists but does not look like original"}}, errors.New("backup exists but does not look like original: " + backup)
		}
	} else {
		if err := os.WriteFile(backup, orig, filePerm(path)); err != nil {
			return patchResult{Entry: logEntry{Level: levelError, Text: "ERROR backup: " + backup}}, err
		}
	}

	replacements := 0
	for _, p := range urlPairs {
		oldUTF8 := []byte(p.Old)
		newUTF8 := []byte(p.New)
		oldUTF16 := utf16LE(p.Old)
		newUTF16 := utf16LE(p.New)

		replacements += bytes.Count(data, oldUTF8)
		replacements += bytes.Count(data, oldUTF16)

		data = bytes.ReplaceAll(data, oldUTF8, newUTF8)
		data = bytes.ReplaceAll(data, oldUTF16, newUTF16)
	}

	if bytes.Equal(data, orig) {
		return patchResult{Entry: logEntry{Level: levelSkip, Text: "NO CHANGE: " + rel}}, nil
	}

	if err := os.WriteFile(path, data, filePerm(path)); err != nil {
		return patchResult{Entry: logEntry{Level: levelError, Text: "ERROR write: " + path}}, err
	}

	if backupExists {
		return patchResult{Entry: logEntry{Level: levelOK, Text: fmt.Sprintf("PATCHED: %s, replacements=%d, existing backup preserved", rel, replacements)}, Patched: true}, nil
	}
	return patchResult{Entry: logEntry{Level: levelOK, Text: fmt.Sprintf("PATCHED: %s, replacements=%d, backup created", rel, replacements)}, Patched: true}, nil
}

func rollbackFile(path string) (rollbackResult, error) {
	rel := displayPath(path)
	backup := path + backupSuffix

	backupData, err := os.ReadFile(backup)
	if errors.Is(err, os.ErrNotExist) {
		return rollbackResult{Entry: logEntry{Level: levelSkip, Text: "NO BACKUP: " + rel}}, nil
	}
	if err != nil {
		return rollbackResult{Entry: logEntry{Level: levelError, Text: "ERROR read backup: " + backup}, BackupSeen: true}, err
	}

	backupOldHits, backupNewHits := countURLHits(backupData)
	if backupOldHits == 0 || backupNewHits > 0 {
		return rollbackResult{Entry: logEntry{Level: levelError, Text: "REFUSE RESTORE: " + rel + " ; backup does not look like original"}, BackupSeen: true}, errors.New("backup does not look like original: " + backup)
	}

	if err := os.WriteFile(path, backupData, filePerm(path)); err != nil {
		return rollbackResult{Entry: logEntry{Level: levelError, Text: "ERROR restore: " + path}, BackupSeen: true}, err
	}

	if err := os.Remove(backup); err != nil {
		return rollbackResult{Entry: logEntry{Level: levelError, Text: "ERROR remove backup after restore: " + backup}, BackupSeen: true}, err
	}

	return rollbackResult{Entry: logEntry{Level: levelOK, Text: "RESTORED AND BACKUP REMOVED: " + rel}, Restored: true, BackupSeen: true}, nil
}

func filePerm(path string) os.FileMode {
	if st, err := os.Stat(path); err == nil {
		return st.Mode().Perm()
	}
	return 0644
}

func countURLHits(data []byte) (oldHits int, newHits int) {
	for _, p := range urlPairs {
		oldHits += bytes.Count(data, []byte(p.Old))
		oldHits += bytes.Count(data, utf16LE(p.Old))
		newHits += bytes.Count(data, []byte(p.New))
		newHits += bytes.Count(data, utf16LE(p.New))
	}
	return oldHits, newHits
}

func findLauncherDLLs(root string) []string {
	root = filepath.Clean(expandHome(root))
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
	for i := 0; i < 4; i++ {
		base := filepath.Base(clean)
		if base == "." || base == string(os.PathSeparator) || base == "" {
			break
		}
		parts = append([]string{base}, parts...)
		clean = filepath.Dir(clean)
	}
	return filepath.Join(parts...)
}

func utf16LE(s string) []byte {
	runes := utf16.Encode([]rune(s))
	buf := make([]byte, len(runes)*2)
	for i, r := range runes {
		binary.LittleEndian.PutUint16(buf[i*2:], r)
	}
	return buf
}
