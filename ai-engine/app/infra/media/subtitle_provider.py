"""Subtitle generation: SRT + ASS.

SRT is the easy interchange format used by backend / frontend preview.
ASS adds typographic styling (font, position, outline) used during
ffmpeg burn-in inside the chapter composer.
"""
from __future__ import annotations

import io
from dataclasses import dataclass


@dataclass(slots=True)
class SubtitleCue:
    index: int
    start_sec: float
    end_sec: float
    text: str


def cues_from_shots(
    shots: list[dict],
    *,
    default_duration: float = 3.0,
) -> list[SubtitleCue]:
    """Build a list of cues from a list of shot dicts.

    Each shot dict may carry `narration`, `duration_sec`, `scene_idx`,
    `shot_idx`. We walk them in order and accumulate timing.
    """
    cues: list[SubtitleCue] = []
    cursor = 0.0
    for i, sh in enumerate(shots, start=1):
        narration = str(sh.get("narration") or "").strip()
        if not narration:
            continue
        dur = float(sh.get("duration_sec") or default_duration)
        # Clamp at 6s per cue for readability.
        dur = max(1.5, min(6.0, dur))
        cues.append(SubtitleCue(index=i, start_sec=cursor, end_sec=cursor + dur, text=narration))
        cursor += dur
    return cues


def render_srt(cues: list[SubtitleCue]) -> bytes:
    buf = io.StringIO()
    for c in cues:
        buf.write(f"{c.index}\n")
        buf.write(f"{_fmt(c.start_sec)} --> {_fmt(c.end_sec)}\n")
        # SRT does not support \n inside cues without an extra blank line trick.
        # Replace newlines with spaces to stay safe.
        buf.write(c.text.replace("\n", " ").strip() + "\n\n")
    return buf.getvalue().encode("utf-8")


# Minimal ASS template. We use Noto Sans CJK if present, else falls back to
# whatever ffmpeg/fontconfig ships; this keeps the bundle small.
_ASS_HEADER = """[Script Info]
Title: novel2av chapter subtitles
ScriptType: v4.00+
PlayResX: 1080
PlayResY: 1920
WrapStyle: 2

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Noto Sans CJK SC,64,&H00FFFFFF,&H000000FF,&H00000000,&H80000000,0,0,0,0,100,100,0,0,1,4,2,2,80,80,160,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
"""


def render_ass(cues: list[SubtitleCue], *, width: int = 1080, height: int = 1920) -> bytes:
    buf = io.StringIO()
    buf.write(_ASS_HEADER)
    for c in cues:
        buf.write(
            f"Dialogue: 0,{_fmt_ass(c.start_sec)},{_fmt_ass(c.end_sec)},Default,,0,0,0,,{_ass_text(c.text)}\n"
        )
    return buf.getvalue().encode("utf-8")


def _fmt_ass(seconds: float) -> str:
    """ASS uses h:mm:ss.cs (centiseconds)."""
    h, rem = divmod(seconds, 3600)
    m, rem = divmod(rem, 60)
    s, cs = divmod(rem, 1)
    return f"{int(h)}:{int(m):02d}:{int(s):02d}.{int(cs * 100):02d}"


def _fmt(seconds: float) -> str:
    """SRT uses hh:mm:ss,mmm."""
    h, rem = divmod(seconds, 3600)
    m, rem = divmod(rem, 60)
    s, ms = divmod(rem, 1)
    return f"{int(h):02d}:{int(m):02d}:{int(s):02d},{int(ms * 1000):03d}"


def _ass_text(s: str) -> str:
    # ASS uses \N for hard newline and {} for override blocks; escape braces.
    return s.replace("\n", r"\N").replace("{", "(").replace("}", ")")
