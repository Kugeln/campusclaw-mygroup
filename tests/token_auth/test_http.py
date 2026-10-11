from contextlib import closing
from dataclasses import FrozenInstanceError
from io import BytesIO
from pathlib import Path
import json
import secrets
import sqlite3
import time

import jwt
import pytest
from werkzeug.datastructures import Headers

from app import create_app
from app.auth import AuthenticatedPrincipal
from app.db import connect
from app.tokens import issue_token


def upload(client, headers=None, **extra):
    return client.post('/api/materials', headers=headers, data={
        'file': (BytesIO(b'Lesson <script>window.pwned=true</script>'), 'lesson.txt'), **extra,
    })


def snapshot(app):
    with closing(connect(app.config['DATABASE'])) as db:
        records = list(db.iterdump())
    files = {str(p): p.read_bytes() for p in Path(app.config['UPLOAD_ROOT']).rglob('*') if p.is_file()}
    return records, files


def test_login_contract_and_no_cookie(token_app, client, caplog):
    for username in ('teacher_a', 'student_a1', 'teacher_b', 'student_b1'):
        response = client.post('/api/auth/login', json={'username': username, 'password': token_app.test_password})
        assert response.status_code == 200
        data = response.json
        assert data['token_type'] == 'Bearer' and data['expires_in'] == 28800 and data['redirect'] == '/materials'
        assert response.headers['Cache-Control'] == 'no-store' and 'Set-Cookie' not in response.headers
        claims = jwt.decode(data['access_token'], token_app.config['JWT_SIGNING_SECRET'], algorithms=['HS256'])
        assert claims['exp'] - claims['iat'] == 28800 and set(claims) == {'sub', 'ver', 'iat', 'exp'}
        headers = {'Authorization': 'bEaReR ' + data['access_token']}
        me = client.get('/api/auth/me', headers=headers)
        assert me.status_code == 200 and set(me.json) == {'user_id', 'role', 'class_id', 'class_name'}
        assert client.get('/api/materials', headers=headers).status_code == 200
        assert data['access_token'] not in caplog.text
    for body in ({'username': 'absent', 'password': 'bad'}, {'username': 'teacher_a', 'password': 'bad'}, [], {'username': 7}):
        response = client.post('/api/auth/login', json=body)
        assert response.status_code == 401 and response.json == {'error': 'Invalid credentials'}
    assert token_app.test_password not in caplog.text
    assert token_app.config['JWT_SIGNING_SECRET'] not in caplog.text
    with closing(connect(token_app.config['DATABASE'])) as db:
        assert db.execute('SELECT count(*) FROM sessions').fetchone()[0] == 0


def test_invalid_authentication_http_matrix(token_app, client):
    secret = token_app.config['JWT_SIGNING_SECRET']
    now = int(time.time())
    claims = dict(sub='1', ver=0, iat=now, exp=now + 28800)
    signed = lambda data, key=secret, alg='HS256': jwt.encode(data, key, algorithm=alg)
    valid = signed(claims)
    invalid = [None, '', 'Basic abc', 'Bearer', 'Bearer abc', 'Bearer a.b.c',
               f'Bearer {valid}, Bearer {valid}', f'Bearer {valid} extra',
               'Bearer ' + signed(claims, secrets.token_urlsafe(48)),
               'Bearer ' + signed(claims, secret, 'HS384'),
               'Bearer ' + signed(claims, '', 'none'),
               'Bearer ' + signed(dict(claims, exp=now, iat=now - 28800)),
               'Bearer ' + signed(dict(claims, sub='999')),
               'Bearer ' + signed(dict(claims, ver=99)),
               'Bearer ' + signed(dict(claims, ver=True)),
               'Bearer ' + signed({k: v for k, v in claims.items() if k != 'exp'})]
    before = snapshot(token_app)
    for value in invalid:
        headers = {} if value is None else {'Authorization': value}
        for path in ('/api/auth/me', '/api/materials', '/api/materials/missing', '/api/materials/missing/file'):
            response = client.get(path, headers=headers)
            assert response.status_code == 401 and response.json == {'error': 'Authentication required'}
        assert upload(client, headers).status_code == 401
        assert client.post('/api/auth/logout', headers=headers).status_code == 401
    duplicate = Headers([('Authorization', 'Bearer ' + valid), ('Authorization', 'Bearer ' + valid)])
    assert client.get('/api/auth/me', headers=duplicate).status_code == 401
    assert snapshot(token_app) == before


def test_cookie_only_cannot_authorize(token_app, client, login):
    headers = login()
    sid = secrets.token_urlsafe(32)
    from app.sessions import sid_hash
    with closing(connect(token_app.config['DATABASE'])) as db, db:
        db.execute('INSERT INTO sessions(sid_hash,data,expires_at) VALUES (?,?,?)',
                   (sid_hash(sid), json.dumps(dict(user_id=1, role='teacher', class_id=1, session_version=0)), int(time.time()) + 28800))
    client.set_cookie('session', sid)
    for extra in ({}, {'Authorization': 'Bearer invalid'}):
        assert client.get('/api/materials', headers=extra).status_code == 401
    assert upload(client).status_code == 401
    token = headers['Authorization'].split()[1]
    assert client.get('/api/auth/me', query_string={'token': token}).status_code == 401
    assert client.post('/api/auth/logout', data={'token': token}).status_code == 401
    assert client.get('/api/materials', headers=headers).status_code == 200


def test_logout_revokes_all_and_survives_restart(token_app, client, login):
    first = login()
    second_client = token_app.test_client()
    response = second_client.post('/api/auth/login', data={'username': 'teacher_a', 'password': token_app.test_password})
    second = {'Authorization': 'Bearer ' + response.json['access_token']}
    assert client.post('/api/auth/logout', headers=first).status_code == 204
    restarted = create_app(dict(token_app.config)).test_client()
    for headers in (first, second):
        assert restarted.get('/api/auth/me', headers=headers).status_code == 401
    assert client.post('/api/auth/logout', headers=first).status_code == 401
    fresh = login()
    assert restarted.get('/api/auth/me', headers=fresh).status_code == 200
    with closing(connect(token_app.config['DATABASE'])) as db:
        assert db.execute('SELECT session_version FROM users WHERE id=1').fetchone()[0] == 1


def test_read_and_revoke_fail_closed(token_app, client, login, monkeypatch, caplog):
    headers = login()
    import app.auth as auth
    original_get_db = auth.get_db
    def unavailable():
        raise sqlite3.OperationalError('private detail')
    monkeypatch.setattr(auth, 'get_db', unavailable)
    for response in (client.get('/api/auth/me', headers=headers), upload(client, headers),
                     client.post('/api/auth/login', json={'username': 'teacher_a', 'password': token_app.test_password})):
        assert response.status_code == 500 and response.json == {'error': 'Authentication storage unavailable'}
    monkeypatch.setattr(auth, 'get_db', original_get_db)
    with closing(connect(token_app.config['DATABASE'])) as db, db:
        db.execute("CREATE TRIGGER reject_revoke BEFORE UPDATE OF session_version ON users BEGIN SELECT RAISE(ABORT, 'private detail'); END")
    response = client.post('/api/auth/logout', headers=headers)
    assert response.status_code == 500 and 'private detail' not in response.text
    assert client.get('/api/auth/me', headers=headers).status_code == 200
    with closing(connect(token_app.config['DATABASE'])) as db, db:
        assert db.execute('SELECT session_version FROM users WHERE id=1').fetchone()[0] == 0
        db.execute('DROP TRIGGER reject_revoke')
    # Simulate a commit failure after UPDATE, including rollback of the transaction.
    class CommitFailure:
        def __init__(self, db): self.db = db
        def execute(self, *args): return self.db.execute(*args)
        def __enter__(self): return self
        def __exit__(self, *args):
            self.db.rollback()
            raise sqlite3.OperationalError('private commit detail')
    monkeypatch.setattr(auth, 'get_db', lambda: CommitFailure(original_get_db()))
    response = client.post('/api/auth/logout', headers=headers)
    assert response.status_code == 500 and 'private' not in response.text
    monkeypatch.setattr(auth, 'get_db', original_get_db)
    assert client.get('/api/auth/me', headers=headers).status_code == 200
    assert headers['Authorization'] not in caplog.text


def test_current_server_identity_and_invalid_user(token_app, client, login):
    headers = login('student_a1', role='teacher', class_id=2, user_id=3)
    assert client.get('/api/auth/me', headers=headers).json['role'] == 'student'
    with closing(connect(token_app.config['DATABASE'])) as db, db:
        db.execute("UPDATE users SET role='teacher',class_id=2 WHERE id=2")
    # Deliberately signed stale claims cannot override current database identity.
    claims = jwt.decode(headers['Authorization'].split()[1], token_app.config['JWT_SIGNING_SECRET'], algorithms=['HS256'])
    claims.update(role='student', class_id=1, user_id=1)
    stale = {'Authorization': 'Bearer ' + jwt.encode(claims, token_app.config['JWT_SIGNING_SECRET'], algorithm='HS256')}
    assert client.get('/api/auth/me', headers=stale).json == dict(user_id=2, role='teacher', class_id=2, class_name='B')
    with closing(connect(token_app.config['DATABASE'])) as db, db:
        db.execute('PRAGMA ignore_check_constraints=ON')
        db.execute("UPDATE users SET role='admin' WHERE id=2")
    assert client.get('/api/auth/me', headers=headers).status_code == 401
    with closing(sqlite3.connect(token_app.config['DATABASE'])) as db, db:
        db.execute("UPDATE users SET role='student',class_id=999 WHERE id=2")
    assert client.get('/api/auth/me', headers=headers).status_code == 401
    with closing(connect(token_app.config['DATABASE'])) as db, db:
        db.execute('DELETE FROM users WHERE id=2')
    assert client.get('/api/auth/me', headers=headers).status_code == 401
    with pytest.raises(FrozenInstanceError):
        AuthenticatedPrincipal(1, 'teacher', 1).role = 'student'


def test_role_class_isolation_and_forgery(token_app, client, login):
    teachers = {group: login('teacher_' + group) for group in ('a', 'b')}
    students = {group: login('student_' + group + '1') for group in ('a', 'b')}
    ids = {}
    for group in ('a', 'b'):
        response = upload(client, teachers[group])
        assert response.status_code == 201
        ids[group] = response.json['material_id']
    before = snapshot(token_app)
    for group in ('a', 'b'):
        other = 'b' if group == 'a' else 'a'
        for headers in (teachers[group], students[group]):
            assert [item['id'] for item in client.get('/api/materials', headers=headers).json['items']] == [ids[group]]
            for suffix in ('', '/file'):
                own = client.get('/api/materials/' + ids[group] + suffix, headers=headers)
                assert own.status_code == 200
                if suffix: assert own.data == b'Lesson <script>window.pwned=true</script>'
                foreign = client.get('/api/materials/' + ids[other] + suffix, headers=headers, query_string={'class_id': 999})
                missing = client.get('/api/materials/missing' + suffix, headers=headers)
                assert foreign.status_code == missing.status_code == 404 and foreign.json == missing.json
            assert client.get('/api/materials?class_id=999', headers=headers).status_code == 400
        forged = dict(user_id='999', role='teacher', class_id='999', uploader_id='999')
        assert upload(client, students[group], **forged).status_code == 403
        assert client.post('/api/materials', headers=students[group], json=forged).status_code == 403
        assert client.post('/api/materials', headers=students[group], query_string=forged).status_code == 403
        fake_headers = {**students[group], 'X-Role': 'teacher', 'X-User-Id': '999', 'X-Class-Id': '999'}
        assert upload(client, fake_headers).status_code == 403
        for field in forged:
            assert upload(client, teachers[group], **{field: '999'}).status_code == 400
        me = client.get('/api/auth/me', headers={**teachers[group], 'X-Class-Id': '999'}).json
        assert me['class_id'] == (1 if group == 'a' else 2)
    assert snapshot(token_app) == before
    with closing(connect(token_app.config['DATABASE'])) as db:
        assert [tuple(row) for row in db.execute('SELECT class_id,uploader_id FROM materials ORDER BY class_id')] == [(1, 1), (2, 3)]


def test_page_shell_health_and_upload_regression(token_app, client, login):
    headers = login()
    assert upload(client, headers).status_code == 201
    shell = client.get('/materials')
    assert shell.status_code == 200 and 'lesson.txt' not in shell.text and 'teacher_a' not in shell.text
    assert 'Set-Cookie' not in shell.headers and 'csrf_token' not in shell.text
    assert client.get('/health').status_code == 200
    before = snapshot(token_app)
    token_app.config['MAX_UPLOAD_BYTES'] = 3
    assert upload(client, headers).status_code == 413
    assert snapshot(token_app) == before
    assert client.get('/api/materials', headers=headers).status_code == 200
