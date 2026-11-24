# TODO

What is not done yet here, and why. When an item is done, remove it from this
file.

## For the maintainer: needs a person, not code

- Watch the first windows-latest Tests run: the SQLite URI fix for a Windows
  drive letter was verified on Linux and with `GOOS=windows go vet` only.
- Push this before caddy-wekan-plugin, whose CI checks out this repository's
  main.
