import secrets

import pytest

from app import create_app
from app.accounts import seed_accounts
from app.db import connect
from app.materials import seed_materials
from scripts.init_db import initialize


@pytest.fixture(autouse=True)
def server_secret(monkeypatch):
    monkeypatch.setenv('SECRET_KEY', secrets.token_urlsafe(48))


@pytest.fixture
def app(tmp_path, monkeypatch):
    password = secrets.token_urlsafe(24)
    monkeypatch.setenv('SECRET_KEY', secrets.token_urlsafe(48))
    database = tmp_path / 'app.db'
    initialize(database)
    with connect(database) as db:
        seed_accounts(db, lambda _: password)
    with connect(database) as db:
        seed_materials(db, tmp_path / 'uploads')
    result = create_app({'TESTING': True, 'DATABASE': str(database), 'UPLOAD_ROOT': str(tmp_path / 'uploads')})
    result.test_password = password
    return result


@pytest.fixture
def client(app):
    return app.test_client()


def sign_in(client, app, username='teacher_a'):
    client.get('/login')
    return client.post('/api/auth/login', json={'username': username, 'password': app.test_password}, headers=csrf_headers(client))


def csrf_headers(client):
    with client.session_transaction() as session:
        return {'X-CSRF-Token': session['csrf_token']}
