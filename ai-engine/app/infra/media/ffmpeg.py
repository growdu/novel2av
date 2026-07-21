"""Thin ffmpeg/ffprobe wrapper using subprocess."""
from __future__ import annotations

import json
import shutil
import subprocess
from dataclasses import dataclass


@dataclass(slots=True)
class ProbeResult:
    duration_sec: float
    width: int
    height: int
    streams: int


def ensure_ffmpeg() -> str:
    path = shutil.which("ffmpeg")
    if not path:
        raise RuntimeError("ffmpeg not found in PATH")
    return path


def run_ffmpeg(args: list[str], timeout: float = 600.0) -> None:
    """Run ffmpeg with the given args. Raises on non-zero exit."""
    cmd = [ensure_ffmpeg(), "-y", *args]
    proc = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
    if proc.returncode != 0:
        raise RuntimeError(f"ffmpeg failed: {proc.stderr[-1000:]}")


def run_ffprobe(path: str) -> ProbeResult:
    out = subprocess.run(
        ["ffprobe", "-v", "error", "-print_format", "json", "-show_format", "-show_streams", path],
        capture_output=True, text=True, check=True,
    )
    data = json.loads(out.stdout)
    duration = float(data.get("format", {}).get("duration", 0.0))
    streams = data.get("streams", [])
    width = 0
    height = 0
    for s in streams:
        if s.get("codec_type") == "video":
            width = int(s.get("width", 0))
            height = int(s.get("height", 0))
            break
    return ProbeResult(duration_sec=duration, width=width, height=height, streams=len(streams))
