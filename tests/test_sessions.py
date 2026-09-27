from contextlib import closing
from pathlib import Path
import time
import json
import re
import sqlite3
from unittest.mock import patch

from flask import request
from flask.sessions import SecureCookieSessionInterface

from app import create_app
from app.db import connect
from app.sessions import sid_hash
from conftest import sign_in, csrf_headers
from scripts.serve import prepare


def test_existing_database_session_preparation(app, monkeypatch):
    monkeypatch.setenv('DATABASE', app.config['DATABASE'])
    monkeypatch.setenv('UPLOAD_ROOT', app.config['UPLOAD_ROOT'])
    root = Path(app.config['UPLOAD_ROOT'])
    files = {str(p): p.read_bytes() for p in root.rglob('*') if p.is_file()}
    with closing(connect(app.config['DATABASE'])) as db:
        with db:
            db.execute('DROP TABLE sessions')
        before = list(db.iterdump())
    prepare()
    with closing(connect(app.config['DATABASE'])) as db:
        assert {r['name'] for r in db.execute('PRAGMA table_info(sessions)')} == {'sid_hash', 'data', 'expires_at'}
        with db:
            db.executemany('INSERT INTO sessions VALUES (?,?,?)', [
                ('expired', '{}', int(time.time()) - 1),
                ('valid', '{}', int(time.time()) + 28800),
            ])
    prepare()
    prepare()
    with closing(connect(app.config['DATABASE'])) as db:
        assert [r['sid_hash'] for r in db.execute('SELECT * FROM sessions')] == ['valid']
        with db:
            db.execute('DROP TABLE sessions')
        assert list(db.iterdump()) == before
    assert {str(p): p.read_bytes() for p in root.rglob('*') if p.is_file()} == files


def test_opaque_persistent_session(app, client):
    assert sign_in(client, app).status_code == 200
    sid = client.get_cookie('session').value
    assert re.fullmatch(r'[A-Za-z0-9_-]{43}', sid)
    with closing(connect(app.config['DATABASE'])) as db:
        row = db.execute('SELECT * FROM sessions WHERE sid_hash=?', (sid_hash(sid),)).fetchone()
        data = json.loads(row['data'])
        assert data['role'] == 'teacher'
        assert isinstance(data['user_id'], int) and isinstance(data['class_id'], int)
        assert sid not in row['data'] and sid != row['sid_hash']
    with client.session_transaction() as session:
        assert dict(session) == data
    restarted = create_app({'TESTING': True, 'DATABASE': app.config['DATABASE'], 'UPLOAD_ROOT': app.config['UPLOAD_ROOT']})
    other = restarted.test_client()
    other.set_cookie('session', sid)
    assert other.get('/api/materials').status_code == 200
    with closing(connect(app.config['DATABASE'])) as db, db:
        db.execute('DELETE FROM sessions WHERE sid_hash=?', (sid_hash(sid),))
    assert other.get('/api/materials').status_code == 401


def test_rotation_and_fixed_expiry(app, client):
    client.get('/login')
    anonymous = client.get_cookie('session').value
    old_csrf = csrf_headers(client)
    assert sign_in(client, app).status_code == 200
    first = client.get_cookie('session').value
    assert first != anonymous and csrf_headers(client) != old_csrf
    assert sign_in(client, app).status_code == 200
    current = client.get_cookie('session').value
    assert current != first
    legacy = SecureCookieSessionInterface().get_signing_serializer(app).dumps(
        {'user_id': 1, 'role': 'teacher', 'class_id': 1, 'session_version': 0})
    for rejected in (anonymous, first, legacy, 'x' * 43):
        client.set_cookie('session', rejected)
        assert client.get('/api/materials').status_code == 401
    client.set_cookie('session', current)
    with closing(connect(app.config['DATABASE'])) as db:
        before = tuple(db.execute('SELECT * FROM sessions WHERE sid_hash=?', (sid_hash(current),)).fetchone())
    expiry = before[2]
    with patch('time.time', return_value=expiry - 1):
        assert client.get('/api/materials').status_code == 200
        assert 'Set-Cookie' not in client.get('/materials').headers
    with closing(connect(app.config['DATABASE'])) as db:
        assert tuple(db.execute('SELECT * FROM sessions WHERE sid_hash=?', (sid_hash(current),)).fetchone()) == before
    with patch('time.time', return_value=expiry):
        assert client.get('/api/materials').status_code == 401


def test_logout_stale_save_cannot_restore_session(app, client):
    sign_in(client, app)
    sid = client.get_cookie('session').value
    with app.test_request_context(headers={'Cookie': f'session={sid}'}):
        stale = app.session_interface.open_session(app, request)
    assert client.post('/api/auth/logout', headers=csrf_headers(client)).status_code == 204
    assert client.get_cookie('session') is None
    stale['csrf_token'] = 'changed-after-logout'
    response = app.response_class()
    app.session_interface.save_session(app, stale, response)
    assert 'Set-Cookie' not in response.headers
    with closing(connect(app.config['DATABASE'])) as db:
        assert db.execute('SELECT 1 FROM sessions WHERE sid_hash=?', (sid_hash(sid),)).fetchone() is None
    client.set_cookie('session', sid)
    assert client.get('/api/materials').status_code == 401


def test_session_read_failure_denies_access(app, client, caplog):
    sign_in(client, app)
    sid = client.get_cookie('session').value
    headers = csrf_headers(client)
    with closing(connect(app.config['DATABASE'])) as db:
        before = list(db.iterdump())
    with patch('app.sessions.connect', side_effect=sqlite3.OperationalError('private database path')):
        for method, path in [('get', '/api/materials'), ('post', '/api/materials'), ('post', '/api/auth/logout')]:
            response = getattr(client, method)(path, headers=headers)
            assert response.status_code == 500
            assert response.json == {'error': 'Session storage unavailable'}
            assert 'Set-Cookie' not in response.headers
        assert client.get('/health').json == {'status': 'ok'}
    with closing(connect(app.config['DATABASE'])) as db:
        assert list(db.iterdump()) == before
    assert 'private database path' not in caplog.text
    assert sid not in caplog.text and app.secret_key not in caplog.text


def test_login_save_failure_is_not_success(app, client, caplog):
    client.get('/login')
    sid = client.get_cookie('session').value
    headers = csrf_headers(client)
    with closing(connect(app.config['DATABASE'])) as db, db:
        db.execute("CREATE TRIGGER reject_session BEFORE INSERT ON sessions BEGIN SELECT RAISE(ABORT, 'private failure'); END")
        before = list(db.iterdump())
    response = client.post('/api/auth/login', json={'username': 'teacher_a', 'password': app.test_password}, headers=headers)
    assert response.status_code == 500
    assert response.json == {'error': 'Session storage unavailable'}
    assert 'Set-Cookie' not in response.headers
    with closing(connect(app.config['DATABASE'])) as db:
        assert list(db.iterdump()) == before  # Rotation deletion rolled back too.
    assert client.get('/api/materials').status_code == 401
    output = response.get_data(as_text=True) + caplog.text
    for secret in (sid, headers['X-CSRF-Token'], app.test_password, app.secret_key, 'private failure'):
        assert secret not in output
