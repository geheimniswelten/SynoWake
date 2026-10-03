#!/usr/bin/env python3
"""Build a reproducible, unsigned DSM 7.1 SPK using only Python's stdlib.

One static Linux/amd64 Go binary provides the package service, an unprivileged
CGI relay and the Task Scheduler/lifecycle CLI.
"""

from __future__ import annotations

import argparse
import gzip
import hashlib
import io
import json
import math
import os
from pathlib import Path
import re
import shutil
import struct
import subprocess
import sys
import tarfile
import zlib

ROOT = Path(__file__).resolve().parents[1]
EPOCH = 0


def png_chunk(kind: bytes, body: bytes) -> bytes:
    return struct.pack(">I", len(body)) + kind + body + struct.pack(">I", zlib.crc32(kind + body))


def rounded_rect(x: float, y: float, left: float, top: float,
                 right: float, bottom: float, radius: float) -> bool:
    if not left <= x <= right or not top <= y <= bottom:
        return False
    cx = min(max(x, left + radius), right - radius)
    cy = min(max(y, top + radius), bottom - radius)
    return (x - cx) ** 2 + (y - cy) ** 2 <= radius ** 2


def icon_pixel(x: float, y: float) -> tuple[int, int, int, int]:
    """A code-native power/network icon, drawn on a 64-unit coordinate grid."""
    if not rounded_rect(x, y, 3, 3, 61, 61, 13):
        return (0, 0, 0, 0)
    blend = (y - 3) / 58
    color = (int(7 + 15 * blend), int(137 - 65 * blend), int(235 - 41 * blend), 255)
    distance = math.hypot(x - 32, y - 27)
    ring = 10.5 <= distance <= 14.5 and not (abs(x - 32) < 5.5 and y < 25)
    bar = rounded_rect(x, y, 29.8, 10.5, 34.2, 27.0, 2.1)
    network = (abs(x - 32) <= 1 and 42 <= y <= 48) or (18 <= x <= 46 and abs(y - 48) <= 1)
    nodes = any(rounded_rect(x, y, cx - 3, 48, cx + 3, 54, 1.5) for cx in (18, 32, 46))
    if ring or bar:
        return (255, 255, 255, 255)
    if network or nodes:
        return (145, 242, 255, 255)
    return color


def icon_png(size: int) -> bytes:
    """Supersampled RGBA PNG; needs no Pillow or image-generation service."""
    rows = bytearray()
    samples = 3
    scale = 64 / size
    for py in range(size):
        rows.append(0)
        for px in range(size):
            rgba = [0, 0, 0, 0]
            for sy in range(samples):
                for sx in range(samples):
                    color = icon_pixel((px + (sx + 0.5) / samples) * scale,
                                       (py + (sy + 0.5) / samples) * scale)
                    for component in range(4):
                        rgba[component] += color[component]
            rows.extend(round(value / (samples * samples)) for value in rgba)
    header = struct.pack(">IIBBBBB", size, size, 8, 6, 0, 0, 0)
    return b"\x89PNG\r\n\x1a\n" + png_chunk(b"IHDR", header) + png_chunk(b"IDAT", zlib.compress(bytes(rows), 9)) + png_chunk(b"IEND", b"")


def normalized(path: Path) -> bytes:
    content = path.read_bytes()
    if path.suffix.lower() in {".html", ".js", ".css", ".json", ".xml", ".sh", ".txt", ".md"} or not path.suffix:
        return content.replace(b"\r\n", b"\n")
    return content


def tar_bytes(files: dict[str, tuple[bytes, int]]) -> bytes:
    output = io.BytesIO()
    directories = set()
    for name in files:
        if name.startswith("/") or ".." in Path(name).parts:
            raise ValueError(f"Unsafe archive path: {name}")
        for parent in Path(name).parents:
            if str(parent) != ".":
                directories.add(parent.as_posix())
    with tarfile.open(fileobj=output, mode="w", format=tarfile.USTAR_FORMAT) as archive:
        for name in sorted(directories):
            entry = tarfile.TarInfo(name + "/")
            entry.type = tarfile.DIRTYPE
            entry.mode = 0o755
            entry.mtime = EPOCH
            entry.uid = entry.gid = 0
            entry.uname = entry.gname = "root"
            archive.addfile(entry)
        for name, (content, mode) in sorted(files.items()):
            entry = tarfile.TarInfo(name)
            entry.size = len(content)
            entry.mode = mode
            entry.mtime = EPOCH
            entry.uid = entry.gid = 0
            entry.uname = entry.gname = "root"
            archive.addfile(entry, io.BytesIO(content))
    return output.getvalue()


def gzip_bytes(content: bytes) -> bytes:
    output = io.BytesIO()
    with gzip.GzipFile(fileobj=output, mode="wb", compresslevel=9, mtime=EPOCH, filename="") as compressed:
        compressed.write(content)
    return output.getvalue()


def check_binary(binary: bytes) -> None:
    if len(binary) < 64 or binary[:6] != b"\x7fELF\x02\x01":
        raise ValueError("Backend must be a little-endian 64-bit Linux ELF, not a Windows executable")
    if struct.unpack_from("<H", binary, 18)[0] != 62:
        raise ValueError("Backend must target amd64/x86-64 for DS918+ (apollolake)")
    # No PT_INTERP: dynamic-loader dependencies would defeat the static package.
    ph_offset = struct.unpack_from("<Q", binary, 32)[0]
    ph_size, ph_count = struct.unpack_from("<HH", binary, 54)
    for number in range(ph_count):
        offset = ph_offset + number * ph_size
        if offset + 4 > len(binary):
            raise ValueError("Invalid ELF program headers")
        if struct.unpack_from("<I", binary, offset)[0] == 3:
            raise ValueError("Backend is dynamically linked; rebuild with CGO_ENABLED=0")


def compile_binary(go_command: str) -> Path:
    binary = ROOT / "build" / "synowake"
    binary.parent.mkdir(parents=True, exist_ok=True)
    executable = shutil.which(go_command) or go_command
    env = dict(os.environ, GOOS="linux", GOARCH="amd64", CGO_ENABLED="0")
    subprocess.run([executable, "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w",
                    "-o", str(binary), "./cmd/synowake"], cwd=ROOT, env=env, check=True)
    return binary


def translation_asset() -> bytes:
    # One source catalog is embedded in Go and exported for the browser.
    catalog = json.loads((ROOT / "internal" / "synowake" / "translations.json").read_text(encoding="utf-8"))
    if not catalog or not all(isinstance(key, str) and isinstance(value, str) and value for key, value in catalog.items()):
        raise ValueError("Invalid translation catalog")
    for key, value in catalog.items():
        if sorted(re.findall(r'\{\d+\}|%[sw]', key)) != sorted(re.findall(r'\{\d+\}|%[sw]', value)):
            raise ValueError(f"Translation parameters differ: {key}")
    return ("// Generated from internal/synowake/translations.json by scripts/build.py.\n"
            "export const english = " + json.dumps(catalog, ensure_ascii=False, sort_keys=True, indent=2) + ";\n").encode("utf-8")


def build(binary_path: Path, output_path: Path | None) -> Path:
    binary = binary_path.read_bytes()
    check_binary(binary)
    package_root = ROOT / "package"
    privilege = json.loads(normalized(package_root / "conf" / "privilege"))
    if privilege != {"defaults": {"run-as": "package"}}:
        raise ValueError("SynoWake must use package privileges only: no executable/tool overrides, setuid or capabilities")
    resource_config = package_root / "conf" / "resource"
    if resource_config.exists() and json.loads(normalized(resource_config)) != {}:
        raise ValueError("Unexpected DSM resource configuration; current SynoWake notifications use application i18n without resource workers")
    ui_root = ROOT / "ui"
    for required in (ui_root / "index.html", ui_root / "app.js", ui_root / "assets" / "synowake.css", ui_root / "scheduler.js"):
        if not required.is_file():
            raise ValueError(f"Missing application asset: {required}")
    for desktop_style in (ui_root / "style.css", package_root / "ui" / "style.css"):
        if desktop_style.exists():
            raise ValueError("Do not expose iframe styles as DSM ui/style.css; use scoped ui/assets/synowake.css instead")

    (ui_root / "translations.js").write_bytes(translation_asset())

    payload: dict[str, tuple[bytes, int]] = {}
    for base, prefix in ((ui_root, "ui"), (package_root / "ui", "ui")):
        for path in sorted(base.rglob("*")):
            if path.is_file():
                name = f"{prefix}/{path.relative_to(base).as_posix()}"
                if name in payload:
                    raise ValueError(f"Duplicate package asset: {name}")
                content = normalized(path)
                if name == "ui/config":
                    json.loads(content)
                payload[name] = (content, 0o644)
    payload["ui/api.cgi"] = (binary, 0o755)
    payload["bin/synowake"] = (binary, 0o755)
    payload["run/.keep"] = (b"", 0o644)
    for size in (16, 24, 32, 48, 64, 72, 128, 256):
        payload[f"ui/images/icon_{size}.png"] = (icon_png(size), 0o644)
    compressed = gzip_bytes(tar_bytes(payload))

    info = normalized(package_root / "INFO").decode("utf-8").rstrip() + "\n"
    if re.search(r'^checksum=', info, re.MULTILINE):
        raise ValueError("INFO checksum is calculated at build time; remove it from the template")
    info += f'checksum="{hashlib.md5(compressed).hexdigest()}"\n'
    version = re.search(r'^version="([\d._-]+)"$', info, re.MULTILINE).group(1)
    for name, references in {
        "ui/index.html": ("assets/synowake.css", "app.js"),
        "ui/app.js": ("./scheduler.js", "./i18n.js"),
        "ui/scheduler.js": ("./i18n.js",),
        "ui/i18n.js": ("./translations.js",),
        "ui/SynoWake.js": ("/webman/3rdparty/SynoWake/index.html",),
    }.items():
        for reference in references:
            if f"{reference}?v={version}".encode() not in payload[name][0]:
                raise ValueError(f"Missing versioned asset reference in {name}: {reference}?v={version}")
    package: dict[str, tuple[bytes, int]] = {
        "INFO": (info.encode("utf-8"), 0o644),
        "package.tgz": (compressed, 0o644),
        "PACKAGE_ICON.PNG": (payload["ui/images/icon_64.png"][0], 0o644),
        "PACKAGE_ICON_256.PNG": (payload["ui/images/icon_256.png"][0], 0o644),
    }
    for directory in ("conf", "scripts"):
        for path in sorted((package_root / directory).rglob("*")):
            if path.is_file():
                content = normalized(path)
                if directory == "conf" and path.name in {"privilege", "resource"}:
                    json.loads(content)
                package[path.relative_to(package_root).as_posix()] = (content, 0o755 if directory == "scripts" else 0o644)
    output_path = output_path or ROOT / "dist" / f"SynoWake-{version}-apollolake.spk"
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_bytes(tar_bytes(package))
    digest = hashlib.sha256(output_path.read_bytes()).hexdigest()
    output_path.with_suffix(output_path.suffix + ".sha256").write_text(f"{digest}  {output_path.name}\n", encoding="ascii")
    print(f"SPK: {output_path}")
    print(f"SHA256: {digest}")
    print("Target: DS918+ / apollolake / DSM >= 7.1-42661 (target NAS validation required)")
    return output_path


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, help="Use an existing static Linux/amd64 ELF instead of compiling")
    parser.add_argument("--go", default="go", help="Go executable used when --binary is omitted")
    parser.add_argument("--output", type=Path, help="Output .spk path; defaults to dist/")
    args = parser.parse_args()
    try:
        binary_path = args.binary or compile_binary(args.go)
        build(binary_path.resolve(), args.output.resolve() if args.output else None)
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"Build failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
