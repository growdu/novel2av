"""MinIO client (S3-compatible) for ai-engine media uploads."""
from __future__ import annotations

from functools import lru_cache

from minio import Minio

from app.settings import get_settings


@lru_cache(maxsize=1)
def get_client() -> Minio:
    s = get_settings()
    return Minio(
        s.s3_endpoint,
        access_key=s.s3_access_key,
        secret_key=s.s3_secret_key,
        secure=s.s3_secure,
        region=s.s3_region,
    )


def put_object(key: str, data: bytes, content_type: str) -> None:
    import io
    s = get_settings()
    get_client().put_object(
        s.s3_bucket,
        key,
        io.BytesIO(data),
        length=len(data),
        content_type=content_type,
    )


def presign_get(key: str, expires_seconds: int = 3600) -> str:
    s = get_settings()
    return get_client().presigned_get_object(s.s3_bucket, key, expires_seconds)
