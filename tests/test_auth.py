"""Legacy auth assertions mapped to the approved Bearer contract."""
from unittest.mock import patch
import time
import pytest
from app import create_app
from app.tokens import verify_token
from conftest import sign_in, auth_headers


def test_login(app, client):
    assert client.get('/login').status_code == 200
    for username, role in [('teacher_a', 'teacher'), ('student_a1', 'student')]:
        assert sign_in(client, app, username).status_code == 200
        me = client.get('/api/auth/me').json
        assert me['role'] == role and me['class_id'] and me['user_id']
    client.environ_base.pop('HTTP_AUTHORIZATION')
    responses = [client.post('/api/auth/login', json={'username': username, 'password': 'wrong'})
                 for username in ('teacher_a', 'missing')]
    assert responses[0].status_code == responses[1].status_code == 401
    assert responses[0].data == responses[1].data
    assert client.get('/api/auth/me').status_code == 401


def test_bearer_contract(app, client, monkeypatch):
    client.set_cookie('session', 'stale_identity')
    response = sign_in(client, app)
    assert 'Set-Cookie' not in response.headers
    assert response.json['expires_in'] == 28800
    token = response.json['access_token']
    claims = verify_token(token, app.config['JWT_SIGNING_SECRET'])
    assert set(claims) == {'sub', 'ver', 'iat', 'exp'}
    assert claims['exp'] - claims['iat'] == 28800
    client.environ_base['HTTP_AUTHORIZATION'] += 'x'
    assert client.get('/api/auth/me').status_code == 401
    client.environ_base['HTTP_AUTHORIZATION'] = 'Bearer ' + token
    with patch('time.time', return_value=time.time() + 28801):
        assert client.get('/api/auth/me').status_code == 401
    monkeypatch.delenv('JWT_SIGNING_SECRET')
    with pytest.raises(RuntimeError, match='JWT_SIGNING_SECRET'):
        create_app()


def assert_protected(client):
    response = client.get('/materials')
    assert response.status_code == 200 and b'Class-A-sample' not in response.data
    for method, path in [('get', '/api/materials'), ('post', '/api/materials'),
                         ('get', '/api/materials/unknown'), ('get', '/api/materials/unknown/file')]:
        assert getattr(client, method)(path).status_code == 401
    assert client.get('/login').status_code == 200


def test_protected_routes(app, client):
    assert_protected(client)
    sign_in(client, app)
    original = client.environ_base['HTTP_AUTHORIZATION']
    client.environ_base['HTTP_AUTHORIZATION'] += 'tampered'
    assert_protected(client)
    client.environ_base['HTTP_AUTHORIZATION'] = original
    with patch('time.time', return_value=time.time() + 28801):
        assert_protected(client)


def test_logout_revokes_all_tokens(app, client):
    other = app.test_client()
    tokens = [sign_in(c, app).json['access_token'] for c in (client, other)]
    assert client.post('/api/auth/logout').status_code == 204
    for token in tokens:
        client.environ_base['HTTP_AUTHORIZATION'] = 'Bearer ' + token
        assert_protected(client)
    for field, value in [('role', 'student'), ('class_id', '999'), ('user_id', '999')]:
        sign_in(client, app)
        client.set_cookie(field, value)
        me = client.get('/api/auth/me', headers={'X-' + field: value}).json
        assert me['role'] == 'teacher' and me['class_id'] != 999 and me['user_id'] != 999


def test_bearer_and_permission_order(app, client, caplog):
    assert client.post('/api/auth/login', json={}).status_code == 401
    assert client.post('/api/materials').status_code == 401
    sign_in(client, app, 'student_a1')
    assert client.post('/api/materials').status_code == 403
    sign_in(client, app)
    token = auth_headers(client)['Authorization']
    responses = []
    for path in ('/api/materials', '/api/auth/logout'):
        for headers in ({'Authorization': ''}, {'Authorization': '', 'X-CSRF-Token': 'forged'}):
            response = client.post(path, headers=headers)
            responses.append(response.get_data(as_text=True))
            assert response.status_code == 401
    assert client.post('/api/materials').status_code == 400
    assert client.post('/api/auth/logout').status_code == 204
    output = ''.join(responses) + caplog.text
    for secret in (app.test_password, app.config['JWT_SIGNING_SECRET'], token, 'pbkdf2:'):
        assert secret not in output