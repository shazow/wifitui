# wifitui 🫣

A replacement for `nmtui`.

`wifitui` is a cute wifi TUI that can:
- [x] **Works with NetworkManager over dbus**
- [x] Show all saved and visible networks
- [x] Fast fuzzy search (`/` to start filtering)
- [x] Show passphrases of known networks
- [x] QR code for sharing a known network with your phone
- [x] Join new and hidden networks (`c` and `n` keys)
- [x] Initiate a scan (`s` key)
- [x] Multiple backends (experimental `iwd` and darwin support, untested)
- [x] Non-interactive modes (`list` `show` `connect` commands)

More things I'd like to do:
- [ ] Bring your own color scheme and theme
- [ ] More stats about the current network
- [ ] Maybe a better name?

## Getting Started

Install [the latest release](https://github.com/shazow/wifitui/releases/) on your fav distro (wifitui is [not maintained package managers yet](https://github.com/shazow/wifitui/issues/48)):

```shell
# Fetch the latest release version
TAG=$(curl -s https://api.github.com/repos/shazow/wifitui/releases/latest | grep "tag_name" | cut -d '"' -f4)

# Arch Linux
sudo pacman -U https://github.com/shazow/wifitui/releases/download/$TAG/wifitui_$TAG_linux_$(uname -m).pkg.tar.zst

# Debian
sudo apt install https://github.com/shazow/wifitui/releases/download/$TAG/wifitui_$TAG_linux_$(uname -m).deb
```


If you have nix, you can run the latest code in one command:

```
nix run github:shazow/wifitui
```


## Acknowldgements

- TUI powered by [bubbletea](https://github.com/charmbracelet/bubbletea).
- Inspired by [impala](https://github.com/pythops/impala).

## License

MIT
