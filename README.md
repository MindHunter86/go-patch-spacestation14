# SS14 URL Patcher

Минимальное GUI-приложение на Go/Fyne для бинарной замены CDN URL в `SS14.Launcher.dll`.

## Что делает

Выбирается папка игры Space Station 14. Далее приложение ищет и патчит:

- `SS14.Launcher_Linux/bin_x64/SS14.Launcher.dll`
- `SS14.Launcher_Linux/bin_arm64/SS14.Launcher.dll`

Перед первым патчем рядом с DLL создается backup:

- `SS14.Launcher.dll.ss220patch.bak`

Откат восстанавливает DLL из этого backup-файла.

## URL

Заменяются пары одинаковой длины:

```text
https://robust-builds.cdn.spacestation14.com/
https://robust.ss14.ss220.club/builds-cdn-v1/

https://robust-builds.fallback.cdn.spacestation14.com/
https://robust-fb.ss14.ss220.club/builds-cdn-fb-v001x/

https://launcher-data.cdn.spacestation14.com/
https://launcher.ss14.ss220.club/data-cdn-v1/

https://launcher-data.fallback.cdn.spacestation14.com/
https://launcher-fb.ss14.ss220.club/data-cdn-fb-v001x/
```

Патчатся ASCII/UTF-8 и UTF-16LE варианты строк.

## Сборка

```bash
go mod tidy
go build -o ss14-url-patcher .
```

Windows:

```powershell
go build -ldflags="-H windowsgui" -o ss14-url-patcher.exe .
```

Linux cross-build с GUI-зависимостями Fyne обычно проще выполнять на Linux с установленными dev-пакетами графического стека.

## Типовые зависимости Linux для Fyne

Debian/Ubuntu:

```bash
sudo apt-get install -y gcc pkg-config libgl1-mesa-dev xorg-dev
```

Fedora:

```bash
sudo dnf install -y gcc pkg-config libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel libglvnd-devel libXxf86vm-devel
```

## Важные ограничения

- Новый URL должен быть строго той же длины, что старый.
- Если Steam обновит игру, DLL может быть перезаписана, и патч нужно применить снова.
- Откат возможен только если backup был создан этим приложением.


## Выбор папки

Патчер принимает не только родительскую папку Steam-игры, но и саму папку лаунчера. Поддерживаемые варианты:

```text
/path/to/Space Station 14
/path/to/Space Station 14/SS14.Launcher_Linux
/path/to/SS14.Launcher_Linux
/path/to/SS14.Launcher_Windows
/path/to/SS14.Launcher.app
```

Приложение ищет `SS14.Launcher.dll` неглубоким рекурсивным поиском и патчит найденные файлы. На Windows папки `SS14.Launcher_Linux` обычно нет; это нормально.
