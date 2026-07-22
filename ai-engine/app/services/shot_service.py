"""Scene breakdown service.

`scene_breakdown` takes a chapter's text + a list of character reference
images and asks the LLM to produce scenes → shots with narration. The
service validates the JSON, normalises durations, and limits shots per
chapter to a sane upper bound.
"""
from __future__ import annotations

import json
import logging
from dataclasses import dataclass, field

from pydantic import BaseModel, Field, ValidationError

from app.infra.llm.gateway import ChatMessage, chat

log = logging.getLogger(__name__)


class ShotDraft(BaseModel):
    idx: int = Field(..., ge=1)
    type: str = Field(default="wide", pattern="^(wide|medium|closeup)$")
    description: str = Field(default="", max_length=400)
    narration: str = Field(default="", max_length=500)
    duration_hint: float = Field(default=3.0, ge=0.5, le=15.0)


class SceneDraft(BaseModel):
    idx: int = Field(..., ge=1)
    location: str = Field(default="")
    mood: str = Field(default="")
    characters: list[str] = Field(default_factory=list)
    shots: list[ShotDraft] = Field(default_factory=list)


class BreakdownResult(BaseModel):
    scenes: list[SceneDraft] = Field(default_factory=list)


@dataclass(slots=True)
class ShotRecord:
    scene_idx: int
    shot_idx: int
    type: str
    description: str
    narration: str
    mood: str
    duration_sec: float
    characters: list[str] = field(default_factory=list)


MAX_SHOTS_PER_CHAPTER = 64
MIN_SHOT_DURATION = 2.5
MAX_SHOT_DURATION = 8.0
TARGET_DURATION = 4.0


def _clamp_duration(value: float, narration: str) -> float:
    """Use narration length to nudge duration when the LLM gave us something implausible."""
    if not narration:
        return max(MIN_SHOT_DURATION, min(MAX_SHOT_DURATION, value))
    # Rough estimate: Chinese ~3.5 chars/sec; latin words ~2.2 words/sec.
    chinese = sum(1 for ch in narration if not ch.isspace() and ord(ch) > 0x4E00)
    latin_words = len([w for w in narration.split() if any(c.isalpha() for c in w)])
    estimate = max(chinese / 3.5, latin_words / 2.2, MIN_SHOT_DURATION)
    if estimate > value:
        return min(MAX_SHOT_DURATION, max(estimate, value))
    return max(MIN_SHOT_DURATION, min(MAX_SHOT_DURATION, value))


async def scene_breakdown(
    text: str,
    *,
    characters: list[str],
    provider: str,
    model: str,
    max_chars: int = 8000,
) -> list[ShotRecord]:
    """Run LLM scene breakdown and return a flat list of shot records."""
    if not text.strip():
        return []

    try:
        system, user_tmpl = _load_prompt()
    except Exception:
        system, user_tmpl = _FALLBACK_SYSTEM, _FALLBACK_USER

    char_block = "\n".join(f"- {n}" for n in characters) or "- （无）"
    user = (user_tmpl
            .replace("{{text}}", text[:max_chars])
            .replace("{{characters}}", char_block))

    res = await chat(provider, model, [
        ChatMessage("system", system),
        ChatMessage("user", user),
    ], response_format_json=True)

    try:
        raw = json.loads(res.content)
    except json.JSONDecodeError:
        cleaned = res.content.strip().strip("`").strip()
        if cleaned.startswith("json"):
            cleaned = cleaned[4:]
        raw = json.loads(cleaned)

    try:
        parsed = BreakdownResult.model_validate(raw)
    except ValidationError as exc:
        log.warning("breakdown validation failed", extra={"err": str(exc)})
        return []

    out: list[ShotRecord] = []
    for s_idx, scene in enumerate(parsed.scenes, start=1):
        for sh in scene.shots:
            out.append(ShotRecord(
                scene_idx=s_idx,
                shot_idx=len(out) + 1,
                type=sh.type,
                description=sh.description,
                narration=sh.narration,
                mood=scene.mood,
                duration_sec=_clamp_duration(sh.duration_hint, sh.narration),
                characters=scene.characters,
            ))
            if len(out) >= MAX_SHOTS_PER_CHAPTER:
                log.warning("shot cap reached", extra={"cap": MAX_SHOTS_PER_CHAPTER})
                return out
    return out


def _load_prompt() -> tuple[str, str]:
    import yaml
    from pathlib import Path
    p = Path(__file__).resolve().parents[1] / "prompts" / "scene_breakdown.yaml"
    with p.open("r", encoding="utf-8") as f:
        data = yaml.safe_load(f)
    return data.get("system", ""), data.get("user_template", "")


_FALLBACK_SYSTEM = "你是影视分镜师。输出严格 JSON。"
_FALLBACK_USER = (
    "把下面章节正文拆解为场景+镜头，每个镜头包含 type/description/narration/duration_hint。\n"
    "正文：\n```\n{text}\n```\n角色：\n{characters}\n"
)
