# Third-party notices

managents itself is MIT licensed (see [LICENSE](LICENSE)). The helper and the display firmware are built
from the open-source components below, and their licenses ask for these notices in binary distributions.
This file ships in every release archive. The versions are those of the build this file belongs to.

## Helper

The helper is a Go program. It contains the Go standard library and runtime, which are BSD-3-Clause
licensed (Copyright The Go Authors), and these modules:

| Module | Version | License | Copyright |
| --- | --- | --- | --- |
| github.com/dustin/go-humanize | v1.0.1 | MIT | 2005-2008 Dustin Sallings |
| github.com/ebitengine/purego | v0.11.1 | Apache-2.0 | see the module's LICENSE |
| github.com/go-ole/go-ole (Windows only) | v1.2.6 | MIT | 2013-2017 Yasuhiro Matsumoto |
| github.com/google/uuid | v1.6.0 | BSD-3-Clause | 2009, 2014 Google Inc. |
| github.com/mattn/go-isatty | v0.0.24 | MIT | Yasuhiro Matsumoto |
| github.com/ncruces/go-strftime | v1.0.0 | MIT | 2022 Nuno Cruces |
| github.com/remyoudompheng/bigfft | v0.0.0-20230129092748-24d4a6f8daec | BSD-3-Clause | 2012 The Go Authors |
| github.com/shirou/gopsutil/v4 | v4.26.9 | BSD-3-Clause | 2014 WAKAYAMA Shirou |
| github.com/tklauser/go-sysconf | v0.4.0 | BSD-3-Clause | 2018-2022 Tobias Klauser |
| github.com/tklauser/numcpus | v0.12.0 | Apache-2.0 | see the module's LICENSE |
| github.com/yusufpapurcu/wmi (Windows only) | v1.2.4 | MIT | 2013 Stack Exchange |
| go.bug.st/serial | v1.8.0 | BSD-3-Clause | 2014-2026 Cristian Maglie |
| golang.org/x/sys | v0.48.0 | BSD-3-Clause | 2009 The Go Authors |
| modernc.org/libc | v1.77.1 | BSD-3-Clause | 2017 The Libc Authors |
| modernc.org/mathutil | v1.7.1 | BSD-3-Clause | 2014 The mathutil Authors |
| modernc.org/memory | v1.12.1 | BSD-3-Clause | 2017 The Memory Authors |
| modernc.org/sqlite | v1.60.1 | BSD-3-Clause | 2017 The Sqlite Authors |

`modernc.org/sqlite` carries a translation of the SQLite C library, which is in the public domain.
`modernc.org/libc` includes code from musl libc (MIT, Copyright 2005-2020 Rich Felker, et al.) and
other permissive sources, listed in its `LICENSE-3RD-PARTY.md`. The complete license file of each module
is in its source tree at the version shown.

## Display firmware

The firmware is built with PlatformIO from the sources in `firmware/`. Every release tag has its full
source in this repository, and `make firmware` rebuilds the image from it.

| Component | Version | License | Copyright |
| --- | --- | --- | --- |
| Arduino-ESP32 core | 2.0.17 | LGPL-2.1-or-later | Espressif Systems and the Arduino-ESP32 contributors |
| ESP-IDF (inside the Arduino-ESP32 libraries) | 4.4.7 | Apache-2.0 | Espressif Systems (Shanghai) |
| LovyanGFX | 1.2.30 | BSD-2-Clause | 2020 lovyan03 |
| ArduinoJson | 7.3.2 | MIT | 2014-2026 Benoit Blanchon |
| FreeSans and FreeSansBold glyphs (GNU FreeFont, as converted by Adafruit GFX) | 9, 12, 18, 24 pt | GPL-3.0-or-later with the font exception | GNU FreeFont contributors |
| Codex icon path (LobeHub icons, `@lobehub/icons-static-svg`) | 1.95.1 | MIT | LobeHub |

**Arduino-ESP32 and relinking.** The firmware image links the Arduino-ESP32 core, which is licensed
under the LGPL version 2.1 or later. The complete corresponding source of the firmware, the build
configuration (`firmware/platformio.ini`, which pins the core through `espressif32@7.1.3`) and the build
scripts are public in this repository for every release. You can modify the core or replace it with a
version of your own, rebuild the firmware with `make firmware`, and flash the result to your display.

**ESP-IDF.** The Arduino-ESP32 libraries include ESP-IDF 4.4.7 (Apache-2.0) and the components it bundles,
among them FreeRTOS (MIT) and newlib (BSD-style licenses). The Xtensa GCC 8.4.0 runtime libraries come
with the GNU GCC Runtime Library Exception.

**LovyanGFX.** Its `license.txt` bundles the notices of the code it derives from, which the firmware may
include in part: Adafruit GFX (BSD, Copyright 2012 Adafruit Industries), TFT_eSPI (FreeBSD license,
Copyright 2020 Bodmer) and Adafruit ILI9341 (MIT, written by Limor Fried and Ladyada for Adafruit
Industries; the text above its license must be included in any redistribution).

**Fonts.** The FreeSans glyph bitmaps in LovyanGFX come from GNU FreeFont through Adafruit GFX's font
converter. FreeFont is GPL-3.0-or-later with an exception that lets a document or program that only embeds
the font be distributed under other terms, so the firmware keeps its own license. FreeFont's source and
license are at https://www.gnu.org/software/freefont/.

**Agent logos.** The Codex logo is rasterized by `firmware/scripts/codex_logo.py` from the Codex icon of
LobeHub's icon set. The marks identify the agents a card belongs to: Claude is a trademark of Anthropic and
Codex of OpenAI, and managents is not affiliated with either.

## License texts

The permissive licenses above share these standard texts. Where a module's own license file differs in
its copyright holder only, the holder is the one in the tables.

### MIT

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and
associated documentation files (the "Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the
following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial
portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT
LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO
EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE
USE OR OTHER DEALINGS IN THE SOFTWARE.

### BSD-3-Clause

Redistribution and use in source and binary forms, with or without modification, are permitted provided
that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this list of conditions and the
   following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice, this list of conditions and
   the following disclaimer in the documentation and/or other materials provided with the distribution.
3. Neither the name of the copyright holder nor the names of its contributors may be used to endorse or
   promote products derived from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED
WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A
PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR
ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED
TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGE.

### BSD-2-Clause

Redistribution and use in source and binary forms, with or without modification, are permitted provided
that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this list of conditions and the
   following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice, this list of conditions and
   the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED
WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A
PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR
ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED
TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGE.

### Apache-2.0 and LGPL-2.1-or-later

The full texts are at https://www.apache.org/licenses/LICENSE-2.0 and
https://www.gnu.org/licenses/old-licenses/lgpl-2.1.html.
