# wifitui 🫣

A replacement for `nmtui`.

`wifitui` is a cute wifi TUI that can:
- [x] **Works with dbus and nm-applet**
- [x] List all known networks
- [x] Fuzzy search for known networks (`/` to start filtering)
- [x] Show current network
- [x] Show passphrases of known networks
- [x] Show non-known visible networks
- [x] Connect to new networks (`c` key)
- [x] Initiate a scan (`s` key)
- [x] QR code for sharing a known network
- [x] Multiple backends (experimental `iwd` support, untested)

More things I'd like to do:
- [ ] macOS support maybe for fun?
- [ ] Non-interactive modes
- [ ] More stats about the current network
- [ ] Maybe a better name?


## Acknowldgements

- Powered by [bubbletea](https://github.com/charmbracelet/bubbletea).
- Inspired by [impala](https://github.com/pythops/impala).
- Initial version prototyped with Gemini 2.5 Pro.

## License

MIT
