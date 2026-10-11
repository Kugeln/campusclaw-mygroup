from dataclasses import dataclass
from functools import wraps
import re
import sqlite3

import jwt
from flask import Blueprint, current_app, g, jsonify, render_template, request
from werkzeug.security import check_password_hash

from app.db import get_db
from app.tokens import LIFETIME_SECONDS, issue_token, verify_token

bp = Blueprint('auth', __name__)


@dataclass(frozen=True)
class AuthenticatedPrincipal:
    user_id: int
    role: str
    class_id: int


def valid_user(user):
    return (user is not None and user['role'] in ('teacher', 'student')
            and type(user['class_id']) is int and user['class_id'] > 0
            and type(user['session_version']) is int
            and 0 <= user['session_version'] < 9223372036854775807)


def unauthorized():
    return jsonify(error='Authentication required'), 401, {'WWW-Authenticate': 'Bearer'}


def authenticated():
    def decorate(view):
        @wraps(view)
        def wrapped(*args, **kwargs):
            values = request.headers.getlist('Authorization')
            match = re.fullmatch(r'Bearer +([^\s,]+)', values[0], re.IGNORECASE) if len(values) == 1 else None
            if match is None:
                return unauthorized()
            try:
                claims = verify_token(match[1], current_app.config['JWT_SIGNING_SECRET'])
            except (jwt.InvalidTokenError, ValueError, TypeError, OverflowError):
                return unauthorized()
            try:
                user = get_db().execute(
                    'SELECT u.id,u.role,u.class_id,u.session_version,c.name AS class_name '
                    'FROM users u JOIN classes c ON c.id=u.class_id WHERE u.id=?',
                    (int(claims['sub']),),
                ).fetchone()
            except sqlite3.Error:
                return jsonify(error='Authentication storage unavailable'), 500
            if not valid_user(user) or user['session_version'] != claims['ver']:
                return unauthorized()
            g.principal = AuthenticatedPrincipal(user['id'], user['role'], user['class_id'])
            g.class_name = user['class_name']
            return view(*args, **kwargs)
        return wrapped
    return decorate


def teacher_required(view):
    @wraps(view)
    def wrapped(*args, **kwargs):
        if g.principal.role != 'teacher':
            return jsonify(error='Teacher role required'), 403
        return view(*args, **kwargs)
    return wrapped


@bp.get('/login')
def login_page():
    return render_template('login.html')


@bp.post('/api/auth/login')
def login():
    data = request.get_json(silent=True) if request.is_json else request.form
    data = data if isinstance(data, dict) or hasattr(data, 'get') else {}
    username, password = data.get('username'), data.get('password')
    user = None
    try:
        if isinstance(username, str) and isinstance(password, str):
            user = get_db().execute(
                'SELECT u.* FROM users u JOIN classes c ON c.id=u.class_id WHERE u.username=?',
                (username,),
            ).fetchone()
    except sqlite3.Error:
        return jsonify(error='Authentication storage unavailable'), 500
    try:
        password_ok = valid_user(user) and check_password_hash(user['password_hash'], password)
    except (ValueError, TypeError):
        password_ok = False
    if not password_ok:
        return jsonify(error='Invalid credentials'), 401
    return jsonify(access_token=issue_token(user['id'], user['session_version'],
                                           current_app.config['JWT_SIGNING_SECRET']),
                   token_type='Bearer', expires_in=LIFETIME_SECONDS, redirect='/materials')


@bp.get('/api/auth/me')
@authenticated()
def me():
    principal = g.principal
    return jsonify(user_id=principal.user_id, role=principal.role,
                   class_id=principal.class_id, class_name=g.class_name)


@bp.post('/api/auth/logout')
@authenticated()
def logout():
    try:
        db = get_db()
        with db:
            result = db.execute('UPDATE users SET session_version=session_version+1 WHERE id=?',
                                (g.principal.user_id,))
            if result.rowcount != 1:
                raise sqlite3.DatabaseError('Revocation did not update a user')
    except sqlite3.Error:
        return jsonify(error='Authentication storage unavailable'), 500
    return '', 204
