"""Independent Token fixtures; never use the legacy Session login helpers."""
from contextlib import closing
from pathlib import Path
import secrets

import pytest
from werkzeug.security import generate_password_hash

from app import create_app
from app.db import connect

ROOT = Path(__file__).resolve().parents[2]


@pytest.fixture
def token_app(tmp_path, monkeypatch):
    secret = secrets.token_urlsafe(48)
    password = secrets.token_urlsafe(24)
    monkeypatch.setenv('JWT_SIGNING_SECRET', secret)
    database = tmp_path / 'token.db'
    uploads = tmp_path / 'uploads'
    for directory in (uploads / 'files', uploads / 'tmp'):
        directory.mkdir(parents=True)
    with closing(connect(database)) as db:
        db.executescript((ROOT / 'app/schema.sql').read_text(encoding='utf-8'))
        db.executescript((ROOT / 'app/session_schema.sql').read_text(encoding='utf-8'))
        # Hash once per isolated fixture; account login still uses the real verifier.
        password_hash = generate_password_hash(password, method='pbkdf2:sha256:1000000')
        with db:
            db.executemany('INSERT INTO classes(id,name) VALUES (?,?)', [(1, 'A'), (2, 'B')])
            db.executemany('INSERT INTO users(id,username,password_hash,role,class_id) VALUES (?,?,?,?,?)', [
                (1, 'teacher_a', password_hash, 'teacher', 1),
                (2, 'student_a1', password_hash, 'student', 1),
                (3, 'teacher_b', password_hash, 'teacher', 2),
                (4, 'student_b1', password_hash, 'student', 2),
            ])
    app = create_app({'TESTING': True, 'DATABASE': str(database), 'UPLOAD_ROOT': str(uploads)})
    app.test_password = password
    return app


@pytest.fixture
def client(token_app):
    return token_app.test_client()


@pytest.fixture
def login(token_app, client):
    def perform(username='teacher_a', **extra):
        response = client.post('/api/auth/login', json={
            'username': username, 'password': token_app.test_password, **extra,
        })
        assert response.status_code == 200
        return {'Authorization': 'Bearer ' + response.json['access_token']}
    return perform
