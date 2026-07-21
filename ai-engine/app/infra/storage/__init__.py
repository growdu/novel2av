"""MinIO client used by ai-engine. Shares credentials with backend."""
from app.infra.storage.minio import get_client, put_object, presign_get

__all__ = ["get_client", "put_object", "presign_get"]
