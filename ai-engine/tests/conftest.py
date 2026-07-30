"""Shared pytest fixtures for the ai-engine test suite."""
from __future__ import annotations

import os
import sys
from pathlib import Path

# Make the ai-engine package importable when pytest is invoked from the
# monorepo root without ``pip install -e``.
ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
