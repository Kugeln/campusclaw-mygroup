"""JWT boundary. Business handlers never inspect token claims."""
import re
import time

import jwt

LIFETIME_SECONDS = 8 * 60 * 60


def validate_secret(secret):
    if not isinstance(secret, str) or not secret.strip() or len(secret.encode('utf-8')) < 32:
        raise RuntimeError('JWT_SIGNING_SECRET must contain at least 32 bytes of random secret material')


def issue_token(user_id, version, secret):
    now = int(time.time())
    return jwt.encode({'sub': str(user_id), 'ver': version, 'iat': now,
                       'exp': now + LIFETIME_SECONDS}, secret, algorithm='HS256')


def verify_token(token, secret):
    if not re.fullmatch(r'[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+', token):
        raise jwt.InvalidTokenError('Invalid credential')
    claims = jwt.decode(token, secret, algorithms=['HS256'], options={
        'require': ['sub', 'ver', 'iat', 'exp'],
    })
    subject = claims['sub']
    if (not isinstance(subject, str) or not re.fullmatch(r'[1-9][0-9]{0,18}', subject)
            or int(subject) > 9223372036854775807
            or any(type(claims[key]) is not int for key in ('ver', 'iat', 'exp'))
            or not 0 <= claims['ver'] < 9223372036854775807
            or claims['iat'] < 0
            or claims['exp'] - claims['iat'] != LIFETIME_SECONDS
            or not claims['iat'] <= time.time() < claims['exp']):
        raise jwt.InvalidTokenError('Invalid claims')
    return claims
