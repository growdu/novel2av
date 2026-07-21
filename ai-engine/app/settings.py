"""Runtime settings for the AI engine, loaded from env / .env via pydantic-settings."""
from __future__ import annotations

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="AI_", env_file=".env", extra="ignore")

    env: str = Field(default="dev")
    log_level: str = Field(default="info")

    # Redis (shared with backend; different DB index isolates the queue namespace)
    redis_url: str = Field(default="redis://localhost:6379/1")

    # Object storage (MinIO/S3)
    s3_endpoint: str = Field(default="localhost:9000")
    s3_access_key: str = Field(default="minioadmin")
    s3_secret_key: str = Field(default="minioadmin")
    s3_bucket: str = Field(default="novel2av")
    s3_region: str = Field(default="us-east-1")
    s3_secure: bool = Field(default=False)

    # AI providers
    llm_providers: dict[str, dict] = Field(default_factory=lambda: {
        "doubao": {"base_url": "https://ark.cn-beijing.volces.com/api/v3"},
        "deepseek": {"base_url": "https://api.deepseek.com"},
        "ollama": {"base_url": "http://localhost:11434/v1"},
    })
    image_providers: dict[str, dict] = Field(default_factory=lambda: {
        "seedream": {"base_url": "https://ark.cn-beijing.volces.com/api/v3"},
    })
    tts_providers: dict[str, dict] = Field(default_factory=lambda: {
        "doubao": {"base_url": ""},
        "edge": {"base_url": ""},
    })

    # Defaults used by tasks when not overridden
    default_image_provider: str = "seedream"
    default_tts_provider: str = "doubao"
    default_aspect: str = "9:16"


_settings: Settings | None = None


def get_settings() -> Settings:
    global _settings
    if _settings is None:
        _settings = Settings()
    return _settings
