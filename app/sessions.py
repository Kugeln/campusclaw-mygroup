"""SQLite sessions. The browser holds only a random lookup credential."""
from contextlib import closing
from datetime import datetime, timezone
from hashlib import sha256
import json
from pathlib import Path
import re
import secrets
import sqlite3
import time

from flask import jsonify, session
from flask.sessions import SessionInterface, SessionMixin
from werkzeug.datastructures import CallbackDict


def sid_hash(sid):
    return sha256(sid.encode('ascii')).hexdigest()


def connect(database):
    # Request handling must never create a missing database.
    db = sqlite3.connect(Path(database).resolve().as_uri() + '?mode=rw', uri=True)
    db.row_factory = sqlite3.Row
    return db


class SQLiteSession(CallbackDict, SessionMixin):
    def __init__(self, data=None, *, sid=None, expires_at=None, storage_error=False):
        def changed(value):
            value.modified = True
        super().__init__(data, changed)
        self.sid = sid
        self.expires_at = expires_at
        self.new = sid is None
        self.modified = False
        self.storage_error = storage_error
        self.previous_sid = None

    def rotate(self):
        self.previous_sid = self.sid
        self.sid = None
        self.expires_at = None
        self.new = True
        self.modified = True


def reject_unavailable_session():
    if session.storage_error:
        return jsonify(error='Session storage unavailable'), 500


class SQLiteSessionInterface(SessionInterface):
    def open_session(self, app, request):
        if request.path == '/health':
            return SQLiteSession()
        sid = request.cookies.get(self.get_cookie_name(app))
        if not isinstance(sid, str) or re.fullmatch(r'[A-Za-z0-9_-]{43}', sid) is None:
            return SQLiteSession()
        try:
            with closing(connect(app.config['DATABASE'])) as db:
                row = db.execute('SELECT data,expires_at FROM sessions WHERE sid_hash=?', (sid_hash(sid),)).fetchone()
        except sqlite3.Error:
            return SQLiteSession(storage_error=True)
        if row is None or row['expires_at'] <= time.time():
            return SQLiteSession()
        try:
            data = json.loads(row['data'])
        except (ValueError, TypeError):
            return SQLiteSession()
        if not isinstance(data, dict):
            return SQLiteSession()
        return SQLiteSession(data, sid=sid, expires_at=row['expires_at'])

    def save_session(self, app, session, response):
        if session.storage_error:
            return
        if session.sid or session:
            response.vary.add('Cookie')
        if not session.modified:
            return
        name = self.get_cookie_name(app)
        cookie_options = dict(
            path=self.get_cookie_path(app), domain=self.get_cookie_domain(app),
            secure=self.get_cookie_secure(app), httponly=self.get_cookie_httponly(app),
            samesite=self.get_cookie_samesite(app),
        )
        try:
            if not session:
                if session.sid:
                    with closing(connect(app.config['DATABASE'])) as db, db:
                        db.execute('DELETE FROM sessions WHERE sid_hash=?', (sid_hash(session.sid),))
                response.delete_cookie(name, **cookie_options)
                return
            now = int(time.time())
            if session.new:
                session.sid = secrets.token_urlsafe(32)
                session.expires_at = now + int(app.permanent_session_lifetime.total_seconds())
            with closing(connect(app.config['DATABASE'])) as db, db:
                if session.new:
                    if session.previous_sid:
                        db.execute('DELETE FROM sessions WHERE sid_hash=?', (sid_hash(session.previous_sid),))
                    db.execute('INSERT INTO sessions(sid_hash,data,expires_at) VALUES (?,?,?)',
                               (sid_hash(session.sid), json.dumps(dict(session)), session.expires_at))
                else:
                    result = db.execute('UPDATE sessions SET data=? WHERE sid_hash=? AND expires_at>?',
                                        (json.dumps(dict(session)), sid_hash(session.sid), now))
                    if result.rowcount == 0:
                        # A stale save must never recreate a deleted/expired ID.
                        return
            session.new = False
            session.modified = False
            session.previous_sid = None
            response.set_cookie(name, session.sid,
                                expires=datetime.fromtimestamp(session.expires_at, timezone.utc), **cookie_options)
        except sqlite3.Error:
            # Flask saves sessions after dispatch: don't let a successful login
            # response escape when persistence failed (nor log sensitive data).
            response.status_code = 500
            response.headers.pop('Set-Cookie', None)
            response.headers.pop('Location', None)
            response.set_data('{"error":"Session storage unavailable"}')
            response.content_type = 'application/json'
