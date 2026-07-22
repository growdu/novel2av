"""Chapter splitting service.

Two strategies:
  1. Rule-based: regex over Chinese chapter markers.
  2. LLM-based: feed the (possibly chunked) text to a provider through
     `app.infra.llm.gateway.chat` and parse the JSON result.

The pipeline is: rule split → LLM verify (only on a sample).
"""
from __future__ import annotations

import json
import re
from dataclasses import dataclass

from app.infra.llm.gateway import ChatMessage, chat

# Matches "第一章", "第1章", "Chapter 1", "CHAPTER II", "卷一", etc.
CHAPTER_RE = re.compile(
    r"(?m)^[ \t\u3000]*"
    r"(?:"
    r"第[一二三四五六七八九十百千零〇两0-9]{1,5}[章回节卷集部]"  # 第N章/回/节/卷/集/部
    r"|第[一二三四五六七八九十]{1,3}[篇]"                            # 第N篇
    r"|Chapter\s+[0-9]+(?:\.[0-9]+)?|CHAPTER\s+[IVXLCDM]+"        # Chapter N / CHAPTER II
    r"|卷[一二三四五六七八九十0-9]{1,3}"                            # 卷N
    r")"
    r"[^\n]{0,80}"                                                   # remainder of the heading line
)


@dataclass(slots=True)
class ChapterSlice:
    index: int
    title: str
    start_offset: int
    end_offset: int


def rule_split(text: str) -> list[ChapterSlice]:
    """Pure-rule split. Always succeeds; may be wrong on unusual novels."""
    matches: list[tuple[int, str]] = []
    for m in CHAPTER_RE.finditer(text):
        # The "title" is the rest of the line after the marker.
        rest = m.group(0).strip()
        title = re.sub(r"\s+", " ", rest)
        if len(title) > 80:
            title = title[:80]
        matches.append((m.start(), title))

    if not matches:
        return []

    slices: list[ChapterSlice] = []
    for i, (start, title) in enumerate(matches):
        end = matches[i + 1][0] if i + 1 < len(matches) else len(text)
        slices.append(ChapterSlice(index=i + 1, title=title or f"第{i+1}章",
                                   start_offset=start, end_offset=end))
    return slices


def _load_prompt_template() -> tuple[str, str]:
    """Load the split_chapters prompt YAML from the prompts directory."""
    import yaml  # type: ignore[import-not-found]
    from pathlib import Path

    p = Path(__file__).resolve().parents[1] / "prompts" / "split_chapters.yaml"
    with p.open("r", encoding="utf-8") as f:
        data = yaml.safe_load(f)
    return data.get("system", ""), data.get("user_template", "")


# Built-in fallback if PyYAML is missing.
_FALLBACK_SYSTEM = "你是中文小说编辑，输出严格的 JSON。"
_FALLBACK_USER = (
    "请按章节切分下面的文本。返回 JSON 数组，每个元素形如：\n"
    '{"index": <int>, "title": "<chapter title>", "start_offset": <int>, "end_offset": <int>}\n'
    "原文：\n```\n{text}\n```"
)


async def llm_split(text: str, *, provider: str, model: str, sample_chars: int = 12000) -> list[ChapterSlice]:
    """Send a sample of the text to the LLM and parse its JSON output.

    `sample_chars` caps how much we ship per call. The LLM only validates the
    rule-split boundaries; for huge novels we run this in a loop on chunks.
    """
    try:
        system, user_tmpl = _load_prompt_template()
    except Exception:
        system, user_tmpl = _FALLBACK_SYSTEM, _FALLBACK_USER

    sample = text[:sample_chars]
    user = user_tmpl.replace("{{text}}", sample)

    res = await chat(provider, model, [
        ChatMessage("system", system),
        ChatMessage("user", user),
    ], response_format_json=True)

    try:
        raw = json.loads(res.content)
        items = raw if isinstance(raw, list) else raw.get("chapters", [])
    except json.JSONDecodeError:
        # The LLM sometimes wraps JSON in ``` fences; strip them.
        cleaned = res.content.strip().strip("`").strip()
        if cleaned.startswith("json"):
            cleaned = cleaned[4:]
        items = json.loads(cleaned)

    out: list[ChapterSlice] = []
    for it in items:
        out.append(ChapterSlice(
            index=int(it["index"]),
            title=str(it.get("title", "")).strip() or f"第{it['index']}章",
            start_offset=int(it.get("start_offset", 0)),
            end_offset=int(it.get("end_offset", len(text))),
        ))
    out.sort(key=lambda c: c.start_offset)
    # Renumber to keep order contiguous even if LLM returned arbitrary indices.
    for i, c in enumerate(out, start=1):
        c.index = i
    return out


def merge(slices: list[ChapterSlice], a: int, b: int) -> list[ChapterSlice]:
    """Merge chapter `a` into `b` (1-based, inclusive)."""
    out: list[ChapterSlice] = []
    skip_range = range(min(a, b), max(a, b) + 1)
    merged_in = False
    for s in slices:
        if s.index in skip_range:
            if not merged_in:
                out.append(s)
                merged_in = True
            else:
                # Extend the previous chapter to cover this one.
                out[-1].end_offset = max(out[-1].end_offset, s.end_offset)
        else:
            out.append(s)
    for i, s in enumerate(out, start=1):
        s.index = i
    return out


def split_at(slices: list[ChapterSlice], chapter_index: int, offset: int) -> list[ChapterSlice]:
    """Insert a new chapter boundary inside `chapter_index` at `offset` (relative to chapter start)."""
    out: list[ChapterSlice] = []
    for s in slices:
        if s.index == chapter_index:
            abs_off = s.start_offset + offset
            head = ChapterSlice(index=s.index, title=s.title,
                                start_offset=s.start_offset, end_offset=abs_off)
            tail = ChapterSlice(index=s.index + 1, title=s.title,
                                start_offset=abs_off, end_offset=s.end_offset)
            out.extend([head, tail])
        else:
            out.append(s)
    for i, s in enumerate(out, start=1):
        s.index = i
    return out
