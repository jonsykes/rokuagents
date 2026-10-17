"""Generates src/ui/codex_logo.hpp, the Codex mark as two coverage maps.

The board cannot draw SVG paths, so the mark is rasterized here once: the
blob, and the `>_` prompt cut out of it, each as a 64x64 map of coverage
(0..255). The firmware scales them to the logo size with a box filter.

The path is the Codex icon of LobeHub's icon set (@lobehub/icons-static-svg
1.95.1, icons/codex.svg, MIT licence, see THIRD_PARTY_NOTICES.md). Codex is a
trademark of OpenAI. Plain Python, no dependencies:

    python3 firmware/scripts/codex_logo.py
"""

import math
import pathlib
import re

PATH = (
    "M8.086.457a6.105 6.105 0 013.046-.415c1.333.153 2.521.72 3.564 1.7a.117.117 0 00.107.029c1.408-.346 2.762-.224 "
    "4.061.366l.063.03.154.076c1.357.703 2.33 1.77 2.918 3.198.278.679.418 1.388.421 2.126a5.655 5.655 0 01-.18 "
    "1.631.167.167 0 00.04.155 5.982 5.982 0 011.578 2.891c.385 1.901-.01 3.615-1.183 5.14l-.182.22a6.063 6.063 0 "
    "01-2.934 1.851.162.162 0 00-.108.102c-.255.736-.511 1.364-.987 1.992-1.199 1.582-2.962 2.462-4.948 2.451-1.583"
    "-.008-2.986-.587-4.21-1.736a.145.145 0 00-.14-.032c-.518.167-1.04.191-1.604.185a5.924 5.924 0 01-2.595-.622 "
    "6.058 6.058 0 01-2.146-1.781c-.203-.269-.404-.522-.551-.821a7.74 7.74 0 01-.495-1.283 6.11 6.11 0 01-.017-3.064"
    ".166.166 0 00.008-.074.115.115 0 00-.037-.064 5.958 5.958 0 01-1.38-2.202 5.196 5.196 0 01-.333-1.589 6.915 "
    "6.915 0 01.188-2.132c.45-1.484 1.309-2.648 2.577-3.493.282-.188.55-.334.802-.438.286-.12.573-.22.861-.304a.129"
    ".129 0 00.087-.087A6.016 6.016 0 015.635 2.31C6.315 1.464 7.132.846 8.086.457zm-.804 7.85a.848.848 0 00-1.473"
    ".842l1.694 2.965-1.688 2.848a.849.849 0 001.46.864l1.94-3.272a.849.849 0 00.007-.854l-1.94-3.393zm5.446 6.24a"
    ".849.849 0 000 1.695h4.848a.849.849 0 000-1.696h-4.848z"
)
VIEWBOX = 24.0
SIZE = 64  # map side in cells
SAMPLES = 8  # samples per cell side
ARGS = {"m": 2, "l": 2, "h": 1, "v": 1, "c": 6, "s": 4, "a": 7, "z": 0}


def tokens(path):
    """Commands and numbers, including SVG's packed forms (".5.5", "01")."""
    number = r"[-+]?(?:\d*\.\d+|\d+\.?)(?:[eE][-+]?\d+)?"
    for match in re.finditer(r"[MmLlHhVvCcSsAaZz]|" + number, path):
        yield match.group(0)


def parse(path):
    """Subpaths as lists of points, curves and arcs flattened."""
    stream = list(tokens(path))
    subpaths, points = [], []
    x = y = start_x = start_y = 0.0
    last_control = None
    i, command = 0, None
    while i < len(stream):
        if stream[i].isalpha():
            command = stream[i]
            i += 1
            if command in "zZ":
                if points:
                    subpaths.append(points)
                points, x, y, last_control = [], start_x, start_y, None
                continue
        lower, relative = command.lower(), command.islower()
        values = []
        for _ in range(ARGS[lower]):
            value = stream[i]
            if lower == "a" and len(values) in (3, 4) and len(value) > 1:
                stream[i] = value[1:]  # packed arc flags: "01" is two flags
                value = value[0]
            else:
                i += 1
            values.append(float(value))
        dx, dy = (x, y) if relative else (0.0, 0.0)
        if lower == "m":
            if points:
                subpaths.append(points)
            x, y = values[0] + dx, values[1] + dy
            start_x, start_y, points = x, y, [(x, y)]
            command = "l" if relative else "L"  # further pairs are line-tos
            last_control = None
        elif lower == "l":
            x, y = values[0] + dx, values[1] + dy
            points.append((x, y))
            last_control = None
        elif lower == "h":
            x = values[0] + dx
            points.append((x, y))
            last_control = None
        elif lower == "v":
            y = values[0] + dy
            points.append((x, y))
            last_control = None
        elif lower in "cs":
            if lower == "c":
                c1 = (values[0] + dx, values[1] + dy)
                rest = values[2:]
            else:
                c1 = (2 * x - last_control[0], 2 * y - last_control[1]) if last_control else (x, y)
                rest = values
            c2 = (rest[0] + dx, rest[1] + dy)
            end = (rest[2] + dx, rest[3] + dy)
            for step in range(1, 17):
                t = step / 16
                u = 1 - t
                points.append((
                    u**3 * x + 3 * u * u * t * c1[0] + 3 * u * t * t * c2[0] + t**3 * end[0],
                    u**3 * y + 3 * u * u * t * c1[1] + 3 * u * t * t * c2[1] + t**3 * end[1],
                ))
            last_control, (x, y) = c2, end
        elif lower == "a":
            end = (values[5] + dx, values[6] + dy)
            points.extend(arc((x, y), values[0], values[1], values[2], values[3], values[4], end))
            x, y = end
            last_control = None
    if points:
        subpaths.append(points)
    return subpaths


def arc(start, rx, ry, rotation, large, sweep, end):
    """Points along an SVG elliptical arc (endpoint to centre form, SVG spec F.6.5)."""
    if rx == 0 or ry == 0:
        return [end]
    phi = math.radians(rotation)
    cos_phi, sin_phi = math.cos(phi), math.sin(phi)
    hx, hy = (start[0] - end[0]) / 2, (start[1] - end[1]) / 2
    x1 = cos_phi * hx + sin_phi * hy
    y1 = -sin_phi * hx + cos_phi * hy
    rx, ry = abs(rx), abs(ry)
    scale = x1 * x1 / (rx * rx) + y1 * y1 / (ry * ry)
    if scale > 1:
        rx, ry = rx * math.sqrt(scale), ry * math.sqrt(scale)
    num = rx * rx * ry * ry - rx * rx * y1 * y1 - ry * ry * x1 * x1
    den = rx * rx * y1 * y1 + ry * ry * x1 * x1
    factor = math.sqrt(max(num / den, 0.0)) * (-1 if large == sweep else 1)
    cx1, cy1 = factor * rx * y1 / ry, -factor * ry * x1 / rx
    cx = cos_phi * cx1 - sin_phi * cy1 + (start[0] + end[0]) / 2
    cy = sin_phi * cx1 + cos_phi * cy1 + (start[1] + end[1]) / 2
    theta = math.atan2((y1 - cy1) / ry, (x1 - cx1) / rx)
    delta = math.atan2((-y1 - cy1) / ry, (-x1 - cx1) / rx) - theta
    if sweep and delta < 0:
        delta += 2 * math.pi
    elif not sweep and delta > 0:
        delta -= 2 * math.pi
    points = []
    for step in range(1, 17):
        angle = theta + delta * step / 16
        ex, ey = rx * math.cos(angle), ry * math.sin(angle)
        points.append((cos_phi * ex - sin_phi * ey + cx, sin_phi * ex + cos_phi * ey + cy))
    return points


def coverage(polygons):
    """A SIZE x SIZE map of how much of each cell the union of polygons covers."""
    grid = SIZE * SAMPLES
    edges = [(a, b) for polygon in polygons for a, b in zip(polygon, polygon[1:] + polygon[:1])]
    cells = [[0] * SIZE for _ in range(SIZE)]
    for row in range(grid):
        sy = (row + 0.5) * VIEWBOX / grid
        # Nonzero winding along the scanline: the crossings with their direction.
        crossings = []
        for (ax, ay), (bx, by) in edges:
            if (ay <= sy < by) or (by <= sy < ay):
                crossings.append((ax + (sy - ay) * (bx - ax) / (by - ay), 1 if by > ay else -1))
        crossings.sort()
        winding, inside_from, spans = 0, None, []
        for cx, direction in crossings:
            before = winding
            winding += direction
            if before == 0 and winding != 0:
                inside_from = cx
            elif before != 0 and winding == 0:
                spans.append((inside_from, cx))
        for column in range(grid):
            sx = (column + 0.5) * VIEWBOX / grid
            if any(a <= sx < b for a, b in spans):
                cells[row // SAMPLES][column // SAMPLES] += 1
    full = SAMPLES * SAMPLES
    return [[round(value * 255 / full) for value in line] for line in cells]


def table(name, cells):
    rows = ",\n".join("    " + ", ".join(f"{v:3d}" for v in line) for line in cells)
    return f"inline constexpr std::uint8_t {name}[kCodexMapSize * kCodexMapSize] = {{\n{rows},\n}};\n"


def main():
    subpaths = parse(PATH)
    blob, prompt = subpaths[0], subpaths[1:]
    header = (
        "#pragma once\n\n"
        "// Generated by firmware/scripts/codex_logo.py from the Codex icon of LobeHub's icon set (MIT).\n"
        "// Do not edit: change the script and run it again.\n\n"
        "#include <cstdint>\n\n"
        "namespace managents::ui {\n\n"
        f"inline constexpr std::int32_t kCodexMapSize = {SIZE};\n\n"
        "// clang-format off\n"
        "/// How much of each cell the blob covers, prompt included (0..255).\n"
        + table("kCodexBlob", coverage([blob]))
        + "\n/// How much of each cell the `>_` prompt cut out of the blob covers (0..255).\n"
        + table("kCodexPrompt", coverage(prompt))
        + "// clang-format on\n"
        + "\n}  // namespace managents::ui\n"
    )
    out = pathlib.Path(__file__).resolve().parent.parent / "src" / "ui" / "codex_logo.hpp"
    out.write_text(header)
    print(f"wrote {out} ({len(subpaths)} subpaths)")


if __name__ == "__main__":
    main()
