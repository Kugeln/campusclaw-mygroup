from pathlib import Path
import sqlite3
from unittest.mock import patch

import pytest

from app.db import connect
from app.recovery import recover_storage
from conftest import sign_in
from test_uploads import snapshot, upload


def test_failed_upload_rolls_back(app, client):
    sign_in(client, app)
    before = snapshot(app)
    with patch('pathlib.Path.replace', side_effect=OSError('storage failure')):
        assert upload(client).status_code == 500
    assert snapshot(app) == before


def test_commit_failure_cleans_moved_file_and_both_rows(app, client):
    sign_in(client, app)
    with connect(app.config['DATABASE']) as db:
        db.execute('CREATE TABLE deferred_failure(class_id INTEGER REFERENCES classes(id) DEFERRABLE INITIALLY DEFERRED)')
        db.execute('CREATE TRIGGER fail_at_commit AFTER INSERT ON knowledge_entries BEGIN INSERT INTO deferred_failure VALUES (-1); END')
    before = snapshot(app)
    assert upload(client).status_code == 500
    assert snapshot(app) == before
    with connect(app.config['DATABASE']) as db:
        db.execute("CREATE TRIGGER reject_text BEFORE INSERT ON knowledge_entries BEGIN SELECT RAISE(ABORT, 'failure'); END")
    before = snapshot(app)
    assert upload(client).status_code == 500
    assert snapshot(app) == before


def test_recover_storage_and_refuse_incomplete(app, monkeypatch):
    from scripts.serve import prepare
    root = Path(app.config['UPLOAD_ROOT'])
    monkeypatch.setenv('DATABASE', app.config['DATABASE'])
    monkeypatch.setenv('UPLOAD_ROOT', str(root))
    before = snapshot(app)
    (root / 'tmp' / 'interrupted').write_bytes(b'incomplete')
    (root / 'files' / 'orphan').write_bytes(b'uncommitted')
    prepare()
    assert snapshot(app) == before
    with connect(app.config['DATABASE']) as db:
        row = db.execute('SELECT id,storage_key FROM materials LIMIT 1').fetchone()
    file = root / 'files' / row['storage_key']
    content = file.read_bytes()
    file.unlink()
    with pytest.raises(RuntimeError, match='Incomplete material'):
        prepare()
    file.write_bytes(content)
    with connect(app.config['DATABASE']) as db:
        db.execute('DELETE FROM knowledge_entries WHERE material_id=?', (row['id'],))
    with pytest.raises(RuntimeError, match='Incomplete material'):
        prepare()
