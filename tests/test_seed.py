import os
from pathlib import Path
import secrets
import sqlite3
import subprocess
import sys

from werkzeug.security import check_password_hash

from app.accounts import SAMPLE_ACCOUNTS


def test_seed_passwords(tmp_path):
    database = tmp_path / 'app.db'
    password = secrets.token_urlsafe(24)
    environment = dict(os.environ, **{row[3]: password for row in SAMPLE_ACCOUNTS})
    script = Path(__file__).resolve().parents[1] / 'scripts/init_db.py'
    uploads = tmp_path / 'uploads'
    command = [sys.executable, str(script), '--database', str(database), '--uploads', str(uploads), '--seed']
    result = subprocess.run(command, env=environment, capture_output=True, text=True, check=True)
    with sqlite3.connect(database) as db:
        rows = db.execute('SELECT u.username,u.role,c.name,u.password_hash FROM users u JOIN classes c ON c.id=u.class_id ORDER BY u.username').fetchall()
    assert {(r[0], r[1], r[2]) for r in rows} == {r[:3] for r in SAMPLE_ACCOUNTS}
    hashes = [r[3] for r in rows]
    assert len(set(hashes)) == 4
    for hashed in hashes:
        assert hashed.startswith('pbkdf2:sha256:')
        assert check_password_hash(hashed, password)
        assert not check_password_hash(hashed, 'incorrect')
        assert hashed not in result.stdout + result.stderr
    assert password not in result.stdout + result.stderr
    assert password.encode() not in database.read_bytes()
    with sqlite3.connect(database) as db:
        before = list(db.iterdump())
        materials = db.execute('SELECT m.original_name,m.storage_key,m.class_id,k.class_id,k.plain_text,c.name FROM materials m JOIN knowledge_entries k ON k.material_id=m.id JOIN classes c ON c.id=m.class_id').fetchall()
    assert len(materials) == 2
    for name, key, class_id, text_class, text, class_name in materials:
        assert class_id == text_class
        assert f'Class-{class_name}' in name
        assert f'Class {class_name}' in text
        assert (uploads / 'files' / key).read_bytes() == text.encode()
    # Re-running needs no password and preserves all records and original hashes.
    clean_environment = {k: v for k, v in os.environ.items() if k not in {r[3] for r in SAMPLE_ACCOUNTS}}
    subprocess.run(command, env=clean_environment, capture_output=True, check=True)
    with sqlite3.connect(database) as db:
        assert list(db.iterdump()) == before
    assert len(list((uploads / 'files').iterdir())) == 2
    assert not list((uploads / 'tmp').iterdir())
