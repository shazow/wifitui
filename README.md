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
- [x] Non-interactive modes (`list` `show` `connect` commands)

More things I'd like to do:
- [ ] Bring your own color scheme
- [ ] macOS support maybe for fun?
- [ ] More stats about the current network
- [ ] Join hidden network
- [ ] Maybe a better name?


## Acknowldgements

- Powered by [bubbletea](https://github.com/charmbracelet/bubbletea).
- Inspired by [impala](https://github.com/pythops/impala).
- Initial version prototyped with Gemini 2.5 Pro.

## License

MIT
