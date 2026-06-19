# Сборка SS14 URL Patcher

Проект использует Fyne, поэтому сборка GUI-приложения идет через CGO. Для надежных релизов лучше собирать нативно под каждую ОС: Windows на Windows, Linux на Linux, macOS на macOS.

## Готовые артефакты

GitHub Actions workflow `.github/workflows/build.yml` собирает:

- `ss14-url-patcher-windows-amd64.zip` — внутри один `ss14-url-patcher.exe`.
- `ss14-url-patcher-linux-amd64.tar.gz` — внутри один исполняемый файл `ss14-url-patcher`.
- `ss14-url-patcher-darwin-amd64.zip` — внутри `SS14 URL Patcher.app` для Intel Mac.
- `ss14-url-patcher-darwin-arm64.zip` — внутри `SS14 URL Patcher.app` для Apple Silicon.

## Запуск workflow

1. Залить проект в GitHub-репозиторий.
2. Убедиться, что файл лежит по пути `.github/workflows/build.yml`.
3. Запустить вручную через `Actions -> build -> Run workflow`, либо создать tag:

```bash
git tag v1.0.0
git push origin v1.0.0
```

## Локальная сборка на текущей ОС

```bash
go mod tidy
./scripts/build-native.sh
```

На Windows лучше собирать из MSYS2 MinGW64 shell или через GitHub Actions workflow.

## Linux build dependencies

Debian/Ubuntu:

```bash
sudo apt-get update
sudo apt-get install -y gcc pkg-config libgl1-mesa-dev xorg-dev
```

Эти зависимости нужны только на машине сборки. Пользователю приложения ставить Go/Fyne не требуется.

## Ограничения

Windows `.exe` — один файл, дополнительных DLL от Go/Fyne обычно не требуется.

macOS `.app` — это нормальный формат GUI-приложения. Без Developer ID подписи и notarization macOS может показать предупреждение Gatekeeper.

Linux "абсолютно один бинарник для всех дистрибутивов без системных библиотек" для Fyne практически нереалистичен, потому что GUI идет через системные X11/OpenGL/desktop-библиотеки. На обычных desktop-системах они обычно уже есть. Если нужен максимально чистый UX на Linux, лучше дополнительно делать AppImage или `.deb`/`.rpm`.
