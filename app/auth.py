from flask import Blueprint, g, jsonify, redirect, render_template, request, session
from functools import wraps
from werkzeug.security import check_password_hash
import sqlite3

from app.db import get_db
from app.csrf import csrf_required, csrf_token
from app.sessions import sid_hash

bp = Blueprint('auth', __name__)


def authenticated(*, page=False):
    def decorate(view):
        @wraps(view)
        def wrapped(*args, **kwargs):
            user_id = session.get('user_id')
            user = None
            if isinstance(user_id, int) and not isinstance(user_id, bool):
                user = get_db().execute('SELECT * FROM users WHERE id=?', (user_id,)).fetchone()
            if user is None or any(session.get(field) != user[field] for field in ('role', 'class_id', 'session_version')):
                session.clear()
                if page:
                    return redirect('/login')
                return jsonify(error='Authentication required'), 401
            g.user = user
            return view(*args, **kwargs)
        return wrapped
    return decorate


def teacher_required(view):
    @wraps(view)
    def wrapped(*args, **kwargs):
        if g.user['role'] != 'teacher':
            return jsonify(error='Teacher role required'), 403
        return view(*args, **kwargs)
    return wrapped


@bp.get('/login')
def login_page():
    return render_template('login.html', csrf_token=csrf_token())


@bp.post('/api/auth/login')
@csrf_required
def login():
    data = request.get_json(silent=True) if request.is_json else request.form
    data = data if isinstance(data, dict) or hasattr(data, 'get') else {}
    username, password = data.get('username'), data.get('password')
    user = None
    if isinstance(username, str) and isinstance(password, str):
        user = get_db().execute('SELECT * FROM users WHERE username=?', (username,)).fetchone()
    session.clear()
    if user is None or not check_password_hash(user['password_hash'], password):
        return jsonify(error='Invalid credentials'), 401
    session.rotate()
    for field in ('id', 'role', 'class_id', 'session_version'):
        session['user_id' if field == 'id' else field] = user[field]
    session.permanent = True
    csrf_token()
    return jsonify(redirect='/materials'), 200


@bp.post('/api/auth/logout')
@authenticated()
@csrf_required
def logout():
    db = get_db()
    try:
        with db:
            db.execute('UPDATE users SET session_version=session_version+1 WHERE id=?', (g.user['id'],))
            db.execute('DELETE FROM sessions WHERE sid_hash=?', (sid_hash(session.sid),))
    except sqlite3.Error:
        return jsonify(error='Session storage unavailable'), 500
    session.clear()
    session.sid = None  # Deletion has already committed with the version change.
    return '', 204
