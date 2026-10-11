import secrets
import time

import jwt
import pytest

from app import create_app
from app.tokens import issue_token, verify_token


@pytest.mark.parametrize('secret', [None, '', ' ', 'x' * 31])
def test_missing_or_short_secret_fails_closed(monkeypatch, secret):
    monkeypatch.delenv('JWT_SIGNING_SECRET', raising=False)
    with pytest.raises(RuntimeError, match='JWT_SIGNING_SECRET'):
        create_app({'JWT_SIGNING_SECRET': secret})


def test_roundtrip():
    secret = secrets.token_urlsafe(48)
    claims = verify_token(issue_token(17, 3, secret), secret)
    assert claims['sub'] == '17' and claims['ver'] == 3
    assert claims['exp'] - claims['iat'] == 28800
    assert set(claims) == {'sub', 'ver', 'iat', 'exp'}


@pytest.mark.parametrize('field,value', [
    ('sub', 1), ('sub', True), ('sub', '01'), ('sub', '-1'), ('sub', '9' * 30),
    ('ver', True), ('ver', '0'), ('ver', -1), ('ver', 1.5),
    ('iat', True), ('iat', '1'), ('iat', 1.5), ('exp', True), ('exp', '1'), ('exp', 1.5),
])
def test_strict_claim_types(field, value):
    secret = secrets.token_urlsafe(48)
    now = int(time.time())
    claims = dict(sub='1', ver=0, iat=now, exp=now + 28800)
    claims[field] = value
    with pytest.raises(jwt.InvalidTokenError):
        verify_token(jwt.encode(claims, secret, algorithm='HS256'), secret)


@pytest.mark.parametrize('missing', ['sub', 'ver', 'iat', 'exp'])
def test_required_claims(missing):
    secret = secrets.token_urlsafe(48)
    now = int(time.time())
    claims = dict(sub='1', ver=0, iat=now, exp=now + 28800)
    del claims[missing]
    with pytest.raises(jwt.InvalidTokenError):
        verify_token(jwt.encode(claims, secret, algorithm='HS256'), secret)


@pytest.mark.parametrize('kind', ['HS384', 'HS512', 'none', 'wrong-signature', 'expired', 'future', 'long-lifetime'])
def test_algorithm_signature_and_time(kind):
    secret = secrets.token_urlsafe(48)
    now = int(time.time())
    claims = dict(sub='1', ver=0, iat=now, exp=now + 28800)
    algorithm, key = 'HS256', secret
    if kind in ('HS384', 'HS512', 'none'):
        algorithm = kind
        if kind == 'none': key = ''
    elif kind == 'wrong-signature': key = secrets.token_urlsafe(48)
    elif kind == 'expired': claims.update(iat=now - 28800, exp=now)
    elif kind == 'future': claims.update(iat=now + 60, exp=now + 28860)
    else: claims['exp'] += 1
    with pytest.raises(jwt.InvalidTokenError):
        verify_token(jwt.encode(claims, key, algorithm=algorithm), secret)
