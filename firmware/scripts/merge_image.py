"""Writes managents-firmware-<env>.bin: everything the board needs, in one image to flash at 0x0.

Bootloader, partition table, boot_app0 and the application go to the offsets PlatformIO's own
upload uses (FLASH_EXTRA_IMAGES and ESP32_APP_OFFSET), so esptool, the helper's flasher or a
release download can flash the board without PlatformIO. The version the build used (see
version.py) is written next to it, to managents-firmware-<env>.version.
"""

Import("env")  # noqa: F821  (provided by PlatformIO)

IMAGE = "$BUILD_DIR/managents-firmware-${PIOENV}"
APP = "$BUILD_DIR/${PROGNAME}.bin"

parts = [*env["FLASH_EXTRA_IMAGES"], ("$ESP32_APP_OFFSET", APP)]
# The ESP32-WROOM-32E's 4 MB flash, in the mode and at the clock PlatformIO's upload uses.
merge = (
    f'"$PYTHONEXE" "$OBJCOPY" --chip esp32 merge_bin -o "{IMAGE}.bin" '
    "--flash_mode dio --flash_freq 40m --flash_size 4MB "
    + " ".join(f'{offset} "{path}"' for offset, path in parts)
)


def write_version(target, source, env):
    with open(env.subst(f"{IMAGE}.version"), "w", encoding="utf-8") as file:
        file.write(env["MANAGENTS_FW_VERSION"] + "\n")


env.AddPostAction(
    APP,
    [
        env.VerboseAction(merge, f"Merging {IMAGE}.bin (version $MANAGENTS_FW_VERSION)"),
        env.VerboseAction(write_version, f"Writing {IMAGE}.version"),
    ],
)
