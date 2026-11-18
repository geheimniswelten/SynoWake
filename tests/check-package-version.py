#!/usr/bin/env python3
"""Check DSM launcher versions in real SPK payloads without compiling Go."""

import hashlib
import importlib.util
import io
import json
from pathlib import Path
import re
import shutil
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]
BINARY = ROOT / "build" / "synowake"


def main():
    spec = importlib.util.spec_from_file_location("build", ROOT / "scripts" / "build.py")
    builder = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(builder)
    with tempfile.TemporaryDirectory() as temporary:
        fixture = Path(temporary)
        shutil.copytree(ROOT / "package", fixture / "package")
        shutil.copytree(ROOT / "ui", fixture / "ui")
        catalog = fixture / "internal" / "synowake"
        catalog.mkdir(parents=True)
        shutil.copy2(ROOT / "internal" / "synowake" / "translations.json", catalog)
        builder.ROOT = fixture
        info_path = fixture / "package" / "INFO"
        info_template = info_path.read_text(encoding="utf-8")
        source_version = re.search(r'^version="([^"\r\n]+)"$', info_template, re.MULTILINE).group(1)
        config_path = fixture / "package" / "ui" / "config"
        original_config = config_path.read_bytes()
        for version in ("9.8.7-1234", "9.8.7-1235"):
            info_path.write_text(info_template.replace(f'version="{source_version}"', f'version="{version}"'), encoding="utf-8")
            for relative in ("ui/index.html", "ui/app.js", "ui/scheduler.js", "ui/i18n.js", "package/ui/SynoWake.js"):
                path = fixture / relative
                text = path.read_text(encoding="utf-8")
                text = re.sub(r'\?v=[\d._-]+', f'?v={version}', text)
                path.write_text(text, encoding="utf-8")
            output = fixture / f"{version}.spk"
            builder.build(BINARY, output)
            first = output.read_bytes()
            builder.build(BINARY, output)
            assert output.read_bytes() == first, "SPK build is not reproducible"
            with tarfile.open(fileobj=io.BytesIO(first)) as package:
                info = package.extractfile("INFO").read().decode("utf-8")
                payload = package.extractfile("package.tgz").read()
            assert f'version="{version}"' in info
            assert f'checksum="{hashlib.md5(payload).hexdigest()}"' in info
            with tarfile.open(fileobj=io.BytesIO(payload), mode="r:gz") as archive:
                config = json.load(archive.extractfile("ui/config"))
            definitions = [entry for group in config.values() for entry in group.values()]
            assert {entry["type"] for entry in definitions} >= {"app", "lib"}
            assert all(entry["version"] == version for entry in definitions)
            assert config_path.read_bytes() == original_config, "Source configuration was changed"
    print("Package launcher version checks passed")


if __name__ == "__main__":
    main()
