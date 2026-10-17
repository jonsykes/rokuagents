# Embedded firmware images

`managents flash` writes the firmware image that is embedded in the helper binary. Release builds copy it here
before `go build`, for each supported board:

```
managents-firmware-<board>.bin       merged image, written at flash offset 0x0
managents-firmware-<board>.version   one line: the firmware version, X.Y.Z or a git-describe string
```

For the e32r40t board, `make firmware` produces both files in `firmware/.pio/build/e32r40t/`.

The files are not committed (see `.gitignore`). A helper built without them works as before; `managents flash`
then needs `--image`.
