from contextlib import closing
from pathlib import Path
import secrets
import subprocess

import pytest

from app.accounts import SAMPLE_ACCOUNTS
from app.db import connect
from scripts.serve import prepare


def test_fresh_start_failure_retry_and_preservation(tmp_path, monkeypatch):
    database, uploads = tmp_path / 'data/app.db', tmp_path / 'uploads'
    monkeypatch.setenv('DATABASE', str(database))
    monkeypatch.setenv('UPLOAD_ROOT', str(uploads))
    for row in SAMPLE_ACCOUNTS:
        monkeypatch.delenv(row[3], raising=False)
    with pytest.raises(subprocess.CalledProcessError):
        prepare()
    assert not database.exists()
    assert not [p for p in uploads.rglob('*') if p.is_file()]
    for row in SAMPLE_ACCOUNTS:
        monkeypatch.setenv(row[3], secrets.token_urlsafe(24))
    prepare()
    with closing(connect(database)) as db:
        assert {r[0] for r in db.execute("SELECT name FROM sqlite_master WHERE type='table'")} == {'classes', 'users', 'materials', 'knowledge_entries', 'assignments', 'assistants', 'skills', 'sessions'}
        assert db.execute('SELECT count(*) FROM users').fetchone()[0] == 4
        assert db.execute('SELECT count(*) FROM materials').fetchone()[0] == 2
        assert db.execute('SELECT count(*) FROM knowledge_entries').fetchone()[0] == 2
        before = list(db.iterdump())
    files = {str(p): p.read_bytes() for p in uploads.rglob('*') if p.is_file()}
    for row in SAMPLE_ACCOUNTS:
        monkeypatch.delenv(row[3])
    prepare()
    with closing(connect(database)) as db:
        assert list(db.iterdump()) == before
    assert {str(p): p.read_bytes() for p in uploads.rglob('*') if p.is_file()} == files
