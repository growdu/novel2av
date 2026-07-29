"""Compose a chapter video from per-shot assets using ffmpeg.

Inputs (one per shot):
  - image (PNG) looped to tts duration
  - tts (WAV)
  - bgm (WAV, optional)

Outputs:
  - videos/<project_id>/<chapter_id>.mp4 (H.264 / AAC, yuv420p, faststart)
  - videos/<project_id>/<chapter_id>.srt
  - videos/<project_id>/<chapter_id>.ass  (burned in)
"""
from __future__ import annotations

import json
import logging
import os
import subprocess
import tempfile
from io import BytesIO

from celery import shared_task

from app.infra.media.subtitle_provider import cues_from_shots, render_ass, render_srt
from app.infra.queue.progress import report_progress
from app.infra.queue.notify import notify_complete
from app.infra.storage import get_client
from app.schemas.payloads import ComposeChapterPayload

log = logging.getLogger(__name__)


@shared_task(name="ai:compose_chapter", bind=True, max_retries=2, default_retry_delay=30)
def compose_chapter(self, payload: dict) -> dict:
    parsed = ComposeChapterPayload.model_validate(payload)
    log.info("compose_chapter start", extra={"chapter_id": parsed.chapter_id})
    report_progress(self.request.id, "running", 0, 4, "fetching shots",
                    project_id=parsed.project_id)

    shots_meta = fetch_shots_meta(parsed.project_id, parsed.chapter_id)
    if not shots_meta:
        log.warning("no shots manifest", extra={"chapter_id": parsed.chapter_id})
        return {"status": "ok", "chapter_id": parsed.chapter_id, "video_key": ""}

    with tempfile.TemporaryDirectory(prefix="n2av_") as tmp:
        # 1. Download per-shot assets to tmp files.
        image_paths, tts_paths, bgm_paths, durations = [], [], [], []
        for i, sh in enumerate(shots_meta, start=1):
            ip = os.path.join(tmp, f"img_{i:03d}.png")
            tp = os.path.join(tmp, f"tts_{i:03d}.wav")
            bp = os.path.join(tmp, f"bgm_{i:03d}.wav")
            _download_to(sh.get("image_key", ""), ip)
            _download_to(sh.get("tts_key", ""), tp)
            _download_to(sh.get("bgm_key", ""), bp)
            image_paths.append(ip)
            tts_paths.append(tp)
            bgm_paths.append(bp)
            durations.append(float(sh.get("duration_sec") or 3.0))

        # 2. Subtitle render.
        cues = cues_from_shots(shots_meta)
        srt_path = os.path.join(tmp, "subs.srt")
        ass_path = os.path.join(tmp, "subs.ass")
        with open(srt_path, "wb") as f:
            f.write(render_srt(cues))
        with open(ass_path, "wb") as f:
            f.write(render_ass(cues))

        report_progress(self.request.id, "running", 2, 4, "rendering ffmpeg")
        out_path = os.path.join(tmp, "chapter.mp4")
        run_ffmpeg_concat(image_paths, tts_paths, bgm_paths, durations,
                          ass_path, out_path, aspect=parsed.aspect)

        # 3. Upload.
        report_progress(self.request.id, "running", 3, 4, "uploading")
        video_key = f"videos/{parsed.project_id}/{parsed.chapter_id}.mp4"
        with open(out_path, "rb") as f:
            _upload(video_key, f.read(), "video/mp4")
        with open(srt_path, "rb") as f:
            _upload(f"videos/{parsed.project_id}/{parsed.chapter_id}.srt",
                    f.read(), "application/x-subrip")
        with open(ass_path, "rb") as f:
            _upload(f"videos/{parsed.project_id}/{parsed.chapter_id}.ass",
                    f.read(), "text/x-ssa")

    duration = sum(c.end_sec - c.start_sec for c in cues)
    report_progress(self.request.id, "success", 4, 4, "done")
    result = {"status": "ok", "chapter_id": parsed.chapter_id,
              "video_key": video_key, "duration_sec": duration}
    notify_complete(self.request.id, {
        "task": self.name,
        "project_id": parsed.project_id,
        "payload": payload,
        "result": result,
    })
    return result


# --- helpers ---------------------------------------------------------------

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


def fetch_shots_meta(project_id: str, chapter_id: str) -> list[dict]:
    """Reads results/<id>/chapters/<cid>/shots.json published by backend."""
    from app.settings import get_settings
    s = get_settings()
    key = f"results/{project_id}/chapters/{chapter_id}/shots.json"
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
        return list(body.get("shots", []))
    except Exception:
        return []

def run_ffmpeg_concat(
    image_paths: list[str],
    tts_paths: list[str],
    bgm_paths: list[str],
    durations: list[float],
    ass_path: str,
    out_path: str,
    *,
    aspect: str,
) -> None:
    if not image_paths:
        raise RuntimeError("no shots to compose")
    width, height = {"9:16": (1080, 1920), "16:9": (1920, 1080)}.get(aspect, (1080, 1920))

    cmd: list[str] = ["ffmpeg", "-y"]
    for ip, tp, bp in zip(image_paths, tts_paths, bgm_paths):
        cmd += ["-loop", "1", "-i", ip, "-i", tp, "-i", bp]

    filters: list[str] = []
    for i, dur in enumerate(durations):
        v = f"[{i*3}:v]scale={width}:{height}:force_original_aspect_ratio=cover,crop={width}:{height},format=yuv420p[v{i}]"
        t = f"[{i*3+1}:a]aresample=44100,apad[t{i}]"
        b = f"[{i*3+2}:a]volume=0.25[bgm{i}]"
        m = f"[t{i}][bgm{i}]amix=inputs=2:duration=first:dropout_transition=0[m{i}]"
        filters += [v, t, b, m]

    v_in = "".join(f"[v{i}]" for i in range(len(durations)))
    filters.append(f"{v_in}concat=n={len(durations)}:v=1:a=0[vc]")
    if len(durations) > 1:
        m_in = "".join(f"[m{i}]" for i in range(len(durations)))
        filters.append(f"{m_in}concat=n={len(durations)}:v=0:a=1[ac]")
    else:
        filters.append("[m0]anull[ac]")

    cmd += [
        "-filter_complex", ";\n".join(filters),
        "-map", "[vc]", "-map", "[ac]",
        "-vf", f"ass={ass_path}",
        "-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "veryfast", "-crf", "22",
        "-c:a", "aac", "-b:a", "192k",
        "-movflags", "+faststart",
        "-shortest",
        out_path,
    ]
    proc = subprocess.run(cmd, capture_output=True, text=True, timeout=900)
    if proc.returncode != 0:
        raise RuntimeError(f"ffmpeg failed: {proc.stderr[-1500:]}")
