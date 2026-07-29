"""Character extraction service.

Given the text of several chapters (or their JSON manifests on MinIO), ask
the LLM to list main characters with appearance / personality / voice
profiles, validate with Pydantic, dedupe by (project_id, name).
"""
from __future__ import annotations

import json
import logging
from typing import Iterable

from pydantic import BaseModel, Field, ValidationError

from app.infra.llm.gateway import ChatMessage, chat

log = logging.getLogger(__name__)


class CharacterProfile(BaseModel):
    name: str = Field(..., min_length=1, max_length=64)
    aliases: list[str] = Field(default_factory=list)
    role: str = Field(default="supporting", pattern="^(protagonist|antagonist|supporting)$")
    appearance: str = Field(default="", max_length=600)
    personality: str = Field(default="", max_length=300)
    voice: str = Field(default="", max_length=200)


async def extract_characters(
    texts: Iterable[str],
    *,
    provider: str,
    model: str,
    max_chars: int = 6000,
) -> list[CharacterProfile]:
    """Send a concatenated text sample to the LLM and parse the JSON output.

    Multiple chapters are concatenated and truncated to `max_chars` runes.
    """
    joined = "\n\n---\n\n".join(t.strip() for t in texts if t.strip())
    if not joined:
        return []
    sample = joined[:max_chars]

    try:
        system, user_tmpl = _load_prompt()
    except Exception:
        system, user_tmpl = _FALLBACK_SYSTEM, _FALLBACK_USER

    user = user_tmpl.replace("{{text}}", sample)
    res = await chat_cached(provider, model, [
        ChatMessage("system", system),
        ChatMessage("user", user),
    ], response_format_json=True)

    try:
        raw = json.loads(res.content)
        items = raw if isinstance(raw, list) else raw.get("characters", [])
    except json.JSONDecodeError:
        cleaned = res.content.strip().strip("`").strip()
        if cleaned.startswith("json"):
            cleaned = cleaned[4:]
        items = json.loads(cleaned)

    out: list[CharacterProfile] = []
    for it in items:
        try:
            out.append(CharacterProfile.model_validate(it))
        except ValidationError as exc:
            log.warning("skip invalid character", extra={"err": str(exc), "item": it})

    # Dedupe by name (case-insensitive); first wins, aliases accumulate.
    seen: dict[str, CharacterProfile] = {}
    for c in out:
        key = c.name.strip().lower()
        if not key:
            continue
        if key in seen:
            for a in c.aliases:
                if a and a not in seen[key].aliases:
                    seen[key].aliases.append(a)
            continue
        seen[key] = c
    return list(seen.values())


# --- prompt loading ---------------------------------------------------------

def _load_prompt() -> tuple[str, str]:
    import yaml
    from pathlib import Path
    p = Path(__file__).resolve().parents[1] / "prompts" / "extract_characters.yaml"
    with p.open("r", encoding="utf-8") as f:
        data = yaml.safe_load(f)
    return data.get("system", ""), data.get("user_template", "")


_FALLBACK_SYSTEM = "你是小说角色分析师。输出严格 JSON。"
_FALLBACK_USER = (
    "请提取下列正文中的主要角色，每个角色形如：\n"
    '{"name":"...", "aliases":["..."], "role":"protagonist|antagonist|supporting",'
    '"appearance":"...", "personality":"...", "voice":"..."}\n\n正文：\n```\n{text}\n```'
)


def merge_into_manifest(
    existing: list[dict], new: list[CharacterProfile]
) -> list[dict]:
    """Merge LLM output into an existing manifest, preserving stable ids.

    `existing` items must have an `id` field (uuid). Matching is by
    (name lower-cased, project); new entries get fresh ids.
    """
    by_key = {(c.get("name", "").strip().lower()): c for c in existing}
    out: list[dict] = []
    for prof in new:
        key = prof.name.strip().lower()
        if key in by_key:
            cur = dict(by_key[key])
            cur.update(prof.model_dump())
            cur["aliases"] = sorted(set(cur.get("aliases", []) + prof.aliases))
            out.append(cur)
            by_key.pop(key)
        else:
            d = prof.model_dump()
            d.setdefault("id", "")  # filled by backend later
            out.append(d)
    # Keep remaining existing entries unchanged.
    out.extend(by_key.values())
    return out
