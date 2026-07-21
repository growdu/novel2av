"""LLM Gateway: provider-agnostic chat/structured-output client.

Default providers (OpenAI-compatible):
  - doubao    (Volcengine Ark)
  - deepseek
  - openai
  - ollama    (local)

All other modules call `chat()` and never reach into provider SDKs directly.
"""
from app.infra.llm.gateway import chat, embed, providers, ChatMessage, ChatResult

__all__ = ["chat", "embed", "providers", "ChatMessage", "ChatResult"]
