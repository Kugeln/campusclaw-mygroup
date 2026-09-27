from functools import wraps
import secrets

from flask import jsonify, request, session


def csrf_token():
    if 'csrf_token' not in session:
        session['csrf_token'] = secrets.token_urlsafe(32)
    return session['csrf_token']


def csrf_required(view):
    @wraps(view)
    def wrapped(*args, **kwargs):
        token = request.headers.get('X-CSRF-Token')
        if token is None:
            data = request.get_json(silent=True) if request.is_json else request.form
            token = data.get('csrf_token') if hasattr(data, 'get') else None
        expected = session.get('csrf_token')
        if not isinstance(token, str) or not isinstance(expected, str) or not secrets.compare_digest(token, expected):
            return jsonify(error='Invalid CSRF token'), 403
        return view(*args, **kwargs)
    return wrapped
