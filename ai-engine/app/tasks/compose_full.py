"""Compose a full-book MP4 by concatenating all chapter videos with title cards.

Pipeline:
  - download each chapter video from MinIO to a tmp dir
  - render a 1.5s title card per chapter (ffmpeg drawtext)
  - build a concat list and use ffmpeg concat demuxer (no re-encode for chapter
    segments; the title cards are re-encoded to match the chapter stream
    profile).
  - upload the result to `videos/<project_id>/full.mp4`.

The manifest is at `results/<project_id>/full.json` (published by backend).
"""
from __future__ import annotations

import json
import logging
import os
import subprocess
import tempfile
from io import BytesIO

from celery import shared_task

from app.infra.queue.progress import report_progress
from app.infra.storage import get_client
from app.schemas.payloads import ComposeFullPayload

log = logging.getLogger(__name__)


@shared_task(name="ai:compose_full", bind=True, max_retries=2, default_retry_delay=30)
def compose_full(self, payload: dict) -> dict:
    parsed = ComposeFullPayload.model_validate(payload)
    log.info("compose_full start", extra={"project_id": parsed.project_id})
    report_progress(self.request.id, "running", 0, 4, "fetching chapters",
                    project_id=parsed.project_id)

    chapters = fetch_chapters_meta(parsed.project_id)
    if not chapters:
        log.warning("no chapter videos", extra={"project_id": parsed.project_id})
        return {"status": "ok", "project_id": parsed.project_id, "video_key": ""}

    title = fetch_project_title(parsed.project_id) or "Untitled"

    report_progress(self.request.id, "running", 1, 4, "rendering title cards")
    with tempfile.TemporaryDirectory(prefix="n2av_full_") as tmp:
        inputs: list[str] = []
        for i, ch in enumerate(chapters, start=1):
            chap_path = os.path.join(tmp, f"chap_{i:03d}.mp4")
            _download_to(ch["video_key"], chap_path)
            if os.path.getsize(chap_path) == 0:
                continue

            card_path = os.path.join(tmp, f"card_{i:03d}.mp4")
            render_title_card(card_path, title=title, chapter_index=i,
                              chapter_title=ch.get("title") or f"Chapter {i}",
                              aspect=ch.get("aspect") or "9:16")
            inputs.append(card_path)
            inputs.append(chap_path)

        if not inputs:
            log.warning("no usable chapters")
            return {"status": "ok", "project_id": parsed.project_id, "video_key": ""}

        report_progress(self.request.id, "running", 2, 4, "ffmpeg concat")
        out_path = os.path.join(tmp, "full.mp4")
        run_ffmpeg_concat(inputs, out_path)

        report_progress(self.request.id, "running", 3, 4, "uploading")
        video_key = f"videos/{parsed.project_id}/full.mp4"
        with open(out_path, "rb") as f:
            _upload(video_key, f.read(), "video/mp4")

    report_progress(self.request.id, "success", 4, 4, "done")
    return {"status": "ok", "project_id": parsed.project_id, "video_key": video_key}


# --- helpers ---------------------------------------------------------------

def fetch_chapters_meta(project_id: str) -> list[dict]:
    from app.settings import get_settings
    s = get_settings()
    key = f"results/{project_id}/full.json"
    try:
        resp = get_client().get_object(s.s3_bucket, key)
    except Exception:
        return []
    try:
        data = resp.read()
    finally:
        resp.close()
        resp.release_conn()
    try:
        body = json.loads(data.decode("utf-8"))
        items = body.get("chapters", [])
        return [c for c in items if c.get("video_key")]
    except Exception:
        return []


def fetch_project_title(project_id: str) -> str | None:
    from app.settings import get_settings
    s = get_settings()
    key = f"results/{project_id}/project.json"
    try:
        resp = get_client().get_object(s.s3_bucket, key)
    except Exception:
        return None
    try:
        data = resp.read()
    finally:
        resp.close()
        resp.release_conn()
    try:
        return str(json.loads(data.decode("utf-8")).get("title", ""))
    except Exception:
        return None


def _download_to(key: str, path: str) -> None:
    if not key:
        open(path, "wb").close()
        return
    from app.settings import get_settings
    s = get_settings()
    try:
        resp = get_client().get_object(s.s3_bucket, key)
    except Exception:
        open(path, "wb").close()
        return
    try:
        data = resp.read()
    finally:
        resp.close()
        resp.release_conn()
    with open(path, "wb") as f:
        f.write(data or b"")


def _upload(key: str, body: bytes, content_type: str) -> None:
    from app.settings import get_settings
    s = get_settings()
    get_client().put_object(s.s3_bucket, key, BytesIO(body), len(body),
                            content_type=content_type)


def render_title_card(out_path: str, *, title: str, chapter_index: int,
                      chapter_title: str, aspect: str) -> None:
    width, height = {"9:16": (1080, 1920), "16:9": (1920, 1080)}.get(aspect, (1080, 1920))
    # Escape colons + apostrophes for ffmpeg drawtext.
    safe_title = title.replace("\\", "\\\\").replace(":", r"\:").replace("'", r"\'")
    safe_chap = chapter_title.replace("\\", "\\\\").replace(":", r"\:").replace("'", r"\'")
    filter_complex = (
        f"color=c=0x101418:size={width}x{height}:duration=1.5:rate=24[bg];"
        f"[bg]drawtext=fontfile=/usr/share/fonts/truetype/noto/NotoSansCJK-Bold.ttc:"
        f"text='{safe_title}':fontcolor=white:fontsize=56:x=(w-text_w)/2:y=h/3,"
        f"drawtext=fontfile=/usr/share/fonts/truetype/noto/NotoSansCJK-Bold.ttc:"
        f"text='Chapter {chapter_index}':fontcolor=#9ec5fe:fontsize=72:x=(w-text_w)/2:y=h/2,"
        f"drawtext=fontfile=/usr/share/fonts/truetype/noto/NotoSansCJK-Bold.ttc:"
        f"text='{safe_chap}':fontcolor=white:fontsize=48:x=(w-text_w)/2:y=2*h/3,"
        f"format=yuv420p"
    )
    cmd = [
        "ffmpeg", "-y",
        "-f", "lavfi", "-i", f"color=c=0x101418:size={width}x{height}:duration=1.5:rate=24",
        "-vf", filter_complex,
        "-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "veryfast", "-crf", "22",
        "-an",
        out_path,
    ]
    proc = subprocess.run(cmd, capture_output=True, text=True, timeout=60)
    if proc.returncode != 0:
        # Fallback: a flat colour card with no drawtext (font missing).
        fallback = [
            "ffmpeg", "-y",
            "-f", "lavfi", "-i", f"color=c=0x101418:size={width}x{height}:duration=1.5:rate=24",
            "-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "veryfast", "-crf", "22",
            "-an",
            out_path,
        ]
        subprocess.run(fallback, capture_output=True, text=True, timeout=60, check=True)


def run_ffmpeg_concat(inputs: list[str], out_path: str) -> None:
    """Concat with the demuxer; chapter segments are assumed to share the
    same codec profile (we re-encoded each chapter with the same ffmpeg
    settings in M5 so this should be a no-recode concat).
    """
    list_path = out_path + ".list"
    with open(list_path, "w", encoding="utf-8") as f:
        for p in inputs:
            # ffmpeg concat demuxer requires single-quote-escaped paths.
            f.write(f"file '{p.replace("'", r"'\''")}'\n")

    cmd = [
        "ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", list_path,
        "-c", "copy",
        "-movflags", "+faststart",
        out_path,
    ]
    proc = subprocess.run(cmd, capture_output=True, text=True, timeout=1800)
    if proc.returncode != 0:
        # Codec mismatch between title cards and chapter videos (rare, but
        # possible if a chapter worker used different settings). Re-encode
        # everything to a uniform profile as a safe fallback.
        cmd2 = [
            "ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", list_path,
            "-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "veryfast", "-crf", "22",
            "-c:a", "aac", "-b:a", "192k",
            "-movflags", "+faststart",
            out_path,
        ]
        proc2 = subprocess.run(cmd2, capture_output=True, text=True, timeout=1800)
        if proc2.returncode != 0:
            raise RuntimeError(f"ffmpeg concat failed: {proc2.stderr[-1500:]}")
