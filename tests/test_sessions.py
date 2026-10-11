"""Historical Session guarantees, equivalently verified using Bearer."""
from contextlib import closing
from pathlib import Path
import sqlite3
from unittest.mock import patch
from app import create_app
from app.db import connect
from app.tokens import verify_token
from conftest import sign_in
from scripts.serve import prepare


def test_existing_database_token_preparation(app, monkeypatch):
    monkeypatch.setenv('DATABASE', app.config['DATABASE'])
    monkeypatch.setenv('UPLOAD_ROOT', app.config['UPLOAD_ROOT'])
    root = Path(app.config['UPLOAD_ROOT'])
    files = {str(p): p.read_bytes() for p in root.rglob('*') if p.is_file()}
    with closing(connect(app.config['DATABASE'])) as db, db:
        db.execute('DROP TABLE sessions')
        before = list(db.iterdump())
    prepare()
    prepare()
    with closing(connect(app.config['DATABASE'])) as db:
        assert list(db.iterdump()) == before
    assert {str(p): p.read_bytes() for p in root.rglob('*') if p.is_file()} == files


def test_persistent_token_and_revocation(app, client):
    token = sign_in(client, app).json['access_token']
    claims = verify_token(token, app.config['JWT_SIGNING_SECRET'])
    assert 'role' not in claims and 'class_id' not in claims
    restarted = create_app({'TESTING': True, 'DATABASE': app.config['DATABASE'], 'UPLOAD_ROOT': app.config['UPLOAD_ROOT']})
    other = restarted.test_client()
    other.environ_base['HTTP_AUTHORIZATION'] = 'Bearer ' + token
    assert other.get('/api/materials').status_code == 200
    with closing(connect(app.config['DATABASE'])) as db, db:
        db.execute('UPDATE users SET session_version=session_version+1 WHERE id=?', (int(claims['sub']),))
    assert other.get('/api/materials').status_code == 401


def test_cookie_rejection_and_fixed_expiry(app, client):
    token = sign_in(client, app).json['access_token']
    claims = verify_token(token, app.config['JWT_SIGNING_SECRET'])
    client.environ_base.pop('HTTP_AUTHORIZATION')
    for cookie in ('anonymous', 'legacy-signed', token, 'x' * 43):
        client.set_cookie('session', cookie)
        assert client.get('/api/materials').status_code == 401
    client.environ_base['HTTP_AUTHORIZATION'] = 'Bearer ' + token
    with patch('time.time', return_value=claims['exp'] - 1):
        assert client.get('/api/materials').status_code == 200
        assert 'Set-Cookie' not in client.get('/materials').headers
    with patch('time.time', return_value=claims['exp']):
        assert client.get('/api/materials').status_code == 401


def test_logout_stale_replay_cannot_restore_identity(app, client):
    token = sign_in(client, app).json['access_token']
    assert client.post('/api/auth/logout').status_code == 204
    client.set_cookie('session', token)
    client.environ_base['HTTP_AUTHORIZATION'] = 'Bearer ' + token
    assert client.get('/api/materials').status_code == 401
    other = create_app({'TESTING': True, 'DATABASE': app.config['DATABASE']}).test_client()
    assert other.get('/api/materials', headers={'Authorization': 'Bearer ' + token}).status_code == 401


def test_identity_read_failure_denies_access(app, client, caplog):
    token = sign_in(client, app).json['access_token']
    with closing(connect(app.config['DATABASE'])) as db:
        before = list(db.iterdump())
    with patch('app.auth.get_db', side_effect=sqlite3.OperationalError('private database path')):
        for method, path in [('get', '/api/materials'), ('post', '/api/materials'), ('post', '/api/auth/logout')]:
            response = getattr(client, method)(path)
            assert response.status_code == 500
            assert response.json == {'error': 'Authentication storage unavailable'}
            assert 'Set-Cookie' not in response.headers
        assert client.get('/health').json == {'status': 'ok'}
    with closing(connect(app.config['DATABASE'])) as db:
        assert list(db.iterdump()) == before
    for secret in ('private database path', token, app.config['JWT_SIGNING_SECRET']):
        assert secret not in caplog.text


def test_login_read_failure_and_logout_commit_failure_are_not_success(app, client, caplog):
    with patch('app.auth.get_db', side_effect=sqlite3.OperationalError('private failure')):
        response = sign_in(client, app)
        assert response.status_code == 500 and 'access_token' not in response.json
    assert client.get('/api/materials').status_code == 401
    token = sign_in(client, app).json['access_token']
    with closing(connect(app.config['DATABASE'])) as db, db:
        db.execute("CREATE TRIGGER reject_revoke BEFORE UPDATE OF session_version ON users BEGIN SELECT RAISE(ABORT, 'private failure'); END")
        before = list(db.iterdump())
    response = client.post('/api/auth/logout')
    assert response.status_code == 500
    assert client.get('/api/materials').status_code == 200
    with closing(connect(app.config['DATABASE'])) as db:
        assert list(db.iterdump()) == before
    for secret in (token, app.test_password, app.config['JWT_SIGNING_SECRET'], 'private failure'):
        assert secret not in response.get_data(as_text=True) + caplog.text