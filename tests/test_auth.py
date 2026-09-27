from conftest import sign_in, csrf_headers
from unittest.mock import patch
import pytest
from app import create_app


def test_login(app, client):
    assert client.get('/login').status_code == 200
    for username, role in [('teacher_a', 'teacher'), ('student_a1', 'student')]:
        assert sign_in(client, app, username).status_code == 200
        with client.session_transaction() as session:
            assert session['role'] == role
            assert session['class_id']
            assert session['user_id']
    responses = []
    for username in ('teacher_a', 'missing'):
        client.get('/login')
        responses.append(client.post('/api/auth/login', json={'username': username, 'password': 'wrong'}, headers=csrf_headers(client)))
        with client.session_transaction() as session:
            assert 'user_id' not in session
    assert responses[0].status_code == responses[1].status_code == 401
    assert responses[0].data == responses[1].data


def test_session_contract(app, client, monkeypatch):
    with client.session_transaction() as session:
        session['stale_identity'] = 'old'
    response = sign_in(client, app)
    cookie = response.headers['Set-Cookie']
    assert all(flag in cookie for flag in ('HttpOnly', 'SameSite=Lax', 'Secure', 'Expires='))
    with client.session_transaction() as session:
        assert set(session) == {'user_id', 'role', 'class_id', 'session_version', '_permanent', 'csrf_token'}
    assert app.permanent_session_lifetime.total_seconds() == 8 * 3600
    value = client.get_cookie('session').value
    with app.test_request_context(headers={'Cookie': f'session={value}x'}):
        from flask import request
        assert not app.session_interface.open_session(app, request)
    import time
    with patch('time.time', return_value=time.time() + 8 * 3600 + 1):
        with app.test_request_context(headers={'Cookie': f'session={value}'}):
            assert not app.session_interface.open_session(app, request)
    monkeypatch.delenv('SECRET_KEY')
    with pytest.raises(RuntimeError, match='SECRET_KEY'):
        create_app({'SECRET_KEY': 'cannot-override-environment'})


def assert_protected(client):
    response = client.get('/materials')
    assert response.status_code == 302 and response.location == '/login'
    for method, path in [('get', '/api/materials'), ('post', '/api/materials'), ('get', '/api/materials/unknown'), ('get', '/api/materials/unknown/file')]:
        assert getattr(client, method)(path).status_code == 401
    assert client.get('/login').status_code == 200


def test_protected_routes(app, client):
    import time
    assert_protected(client)
    sign_in(client, app)
    cookie = client.get_cookie('session').value
    client.set_cookie('session', cookie + 'tampered')
    assert_protected(client)
    client.set_cookie('session', cookie)
    with patch('time.time', return_value=time.time() + 28801):
        assert_protected(client)


def test_logout_revokes_all_cookies(app, client):
    other = app.test_client()
    sign_in(client, app)
    sign_in(other, app)
    cookies = [c.get_cookie('session').value for c in (client, other)]
    response = client.post('/api/auth/logout', headers=csrf_headers(client))
    assert response.status_code == 204
    assert client.get_cookie('session') is None
    for value in cookies:
        client.set_cookie('session', value)
        assert client.get('/api/materials').status_code == 401
    for field, forged in [('role', 'student'), ('class_id', 999), ('session_version', 999), ('user_id', 999)]:
        sign_in(client, app)
        with client.session_transaction() as session:
            session[field] = forged
        assert_protected(client)


def test_csrf_and_permission_order(app, client, caplog):
    responses = []
    assert client.post('/api/auth/login', json={}).status_code == 403
    assert client.post('/api/materials').status_code == 401
    sign_in(client, app, 'student_a1')
    assert client.post('/api/materials').status_code == 403
    sign_in(client, app)
    for path in ('/api/materials', '/api/auth/logout'):
        for headers in ({}, {'X-CSRF-Token': 'wrong'}):
            response = client.post(path, headers=headers)
            responses.append(response.get_data(as_text=True))
            assert response.status_code == 403
    assert client.post('/api/materials', headers=csrf_headers(client)).status_code == 400
    assert client.post('/api/auth/logout', headers=csrf_headers(client)).status_code == 204
    output = ''.join(responses) + caplog.text
    assert app.test_password not in output
    assert app.secret_key not in output
    assert 'pbkdf2:' not in output
    assert 'session=' not in output
