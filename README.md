# wifitui 🛜✨

`wifitui` is a fast, featureful, and friendly replacement for `nmtui`.

<img width="814.5" height="369" alt="image" src="https://github.com/user-attachments/assets/2a49cc88-4ce0-4532-b7ef-e64d7c3dc888" />


## Features

- [x] **Works with NetworkManager over dbus**
- [x] Show all saved and visible networks
- [x] Fast fuzzy search (`/` to start filtering)
- [x] Show passphrases of known networks
- [x] QR code for sharing a known network with your phone
- [x] Join new and hidden networks (`c` and `n` keys)
- [x] Initiate a scan (`s` key)
- [x] Multiple backends (experimental `iwd` and darwin support, untested)
- [x] Non-interactive modes (`list` `show` `connect` commands)
- [x] Bring your own color scheme and theme (`--theme=./theme.toml` or set `WIFITUI_THEME=./theme.toml`)

## Getting Started

[![Packaging status](https://repology.org/badge/vertical-allrepos/wifitui.svg)](https://repology.org/project/wifitui/versions)

Install [the latest release](https://github.com/shazow/wifitui/releases/) on your fav distro (wifitui is [not in all package managers yet](https://github.com/shazow/wifitui/issues/48)), here's a handy script for convenience:

```shell
# Fetch the latest release version
TAG=$(curl -s https://api.github.com/repos/shazow/wifitui/releases/latest | grep "tag_name" | cut -d '"' -f4)
OS="linux-$(uname -m)" # x86_64 or arm64
LATEST_RELEASE="https://github.com/shazow/wifitui/releases/download/${TAG}/wifitui-${TAG:1}-${OS}"

# Just the binary (any distro)
wget -q -O- "${LATEST_RELEASE}.tar.gz" | tar xzv

# Debian
sudo apt install "${LATEST_RELEASE}.deb"

# Arch Linux (from AUR)
yay -S wifitui-bin

# Arch Linux (latest release from this repo)
sudo pacman-key --recv-keys 065D66BF7EFEB02BCDC75FF6227578D96B6A5E4C
sudo pacman-key --lsign-key 065D66BF7EFEB02BCDC75FF6227578D96B6A5E4C
sudo pacman -U "${LATEST_RELEASE}.pkg.tar.zst"

# Homebrew for Linux and macOS (experimental)
brew install wifitui
```


If you have nix, you can run the latest code in one command:

```
nix run github:shazow/wifitui
```

Run the TUI:

```
$ wifitui
```

Or run it in non-interactive mode:

```console
$ ./wifitui --help
USAGE
  wifitui [flags] <subcommand> [args...]

SUBCOMMANDS
  list     List wifi networks
  show     Show a wifi network
  connect  Connect to a wifi network

FLAGS
  -version=false  display version

$ ./wifitui show --json "GET off my LAN"
{
  "SSID": "GET off my LAN",
  "IsActive": false,
  "IsKnown": false,
  "IsSecure": false,
  "IsVisible": false,
  "IsHidden": false,
  "Strength": 0,
  "Security": 3,
  "LastConnected": null,
  "AutoConnect": false
}
```

##  Why not `nmtui` or `impala`?

Each has features the other lacks: `nmtui` can reveal passphrases but can't trigger a rescan, `impala` can rescan but can't manage saved networks (partly due to being iwd-exclusive), etc. I used both for a while, but I just wanted one tool that does everything, plus sort by recency, fuzzy filtering, QR code for sharing the network, support multiple backends (nm and iwd), and more.

## Acknowledgement

- TUI powered by [bubbletea](https://github.com/charmbracelet/bubbletea).
- Inspired by [impala](https://github.com/pythops/impala).
- Early versions made possible by Neovim, LSP, Gemini 2.5 Pro, Jules, Github code search, Google, Go, water, oxygen, my Framework laptop running NixOS, the public goods built by socialism, the economies scaled by capitalism, the lands stolen by imperialism, and everything else.

## License

MIT
