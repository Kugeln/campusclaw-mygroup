"""Application factory; importing this package never creates a database."""

from pathlib import Path
import os
from datetime import timedelta

from flask import Flask


def create_app(test_config=None):
    app = Flask(__name__)
    app.config.from_mapping(
        SECRET_KEY=os.environ.get('SECRET_KEY'),
        PERMANENT_SESSION_LIFETIME=timedelta(hours=8),
        SESSION_COOKIE_HTTPONLY=True,
        SESSION_COOKIE_SAMESITE='Lax',
        SESSION_COOKIE_SECURE=os.environ.get('COOKIE_SECURE', 'true').lower() != 'false',
        SESSION_REFRESH_EACH_REQUEST=False,
        MAX_UPLOAD_BYTES=int(os.environ.get('MAX_UPLOAD_BYTES', 20 * 1024 * 1024)),
        DATABASE=str(Path(os.environ.get('DATABASE', 'data/app.db')).resolve()),
        UPLOAD_ROOT=str(Path(os.environ.get('UPLOAD_ROOT', 'uploads')).resolve()),
    )
    if test_config:
        app.config.update(test_config)
    app.config['SECRET_KEY'] = os.environ.get('SECRET_KEY')
    if not app.config['SECRET_KEY']:
        raise RuntimeError('Missing required configuration: SECRET_KEY')
    from app.sessions import SQLiteSessionInterface, reject_unavailable_session
    app.session_interface = SQLiteSessionInterface()
    app.before_request(reject_unavailable_session)
    from app.db import close_db
    app.teardown_appcontext(close_db)
    from app.auth import bp
    app.register_blueprint(bp)
    from app.routes import bp as materials_bp
    app.register_blueprint(materials_bp)
    from app.health import bp as health_bp
    app.register_blueprint(health_bp)
    return app
