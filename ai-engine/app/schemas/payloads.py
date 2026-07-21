"""Schemas matching backend domain types — kept hand-written so ai-engine
has zero coupling to backend internals."""
from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field


class PipelineOptions(BaseModel):
    image_provider: str | None = None
    tts_provider: str | None = None
    aspect: Literal["9:16", "16:9"] | None = None


class SplitChaptersPayload(BaseModel):
    project_id: str
    source_key: str = Field(..., description="MinIO key of the raw novel text")


class ExtractCharactersPayload(BaseModel):
    project_id: str
    chapter_keys: list[str]


class CharacterImagePayload(BaseModel):
    project_id: str
    character_id: str
    variants: int = 4


class SceneBreakdownPayload(BaseModel):
    project_id: str
    chapter_id: str
    character_refs: list[str] = Field(default_factory=list)


class GenerateShotPayload(BaseModel):
    project_id: str
    chapter_id: str
    shot_id: str
    description: str
    narration: str
    mood: str = ""
    character_refs: list[str] = Field(default_factory=list)
    style: str = "cinematic"
    aspect: Literal["9:16", "16:9"] = "9:16"


class ComposeChapterPayload(BaseModel):
    project_id: str
    chapter_id: str
    aspect: Literal["9:16", "16:9"] = "9:16"


class ComposeFullPayload(BaseModel):
    project_id: str
