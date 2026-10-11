"""Application factory; importing this package never creates a database."""

from pathlib import Path
import os

from flask import Flask


def create_app(test_config=None):
    app = Flask(__name__)
    app.config.from_mapping(
        JWT_SIGNING_SECRET=os.environ.get('JWT_SIGNING_SECRET'),
        MAX_UPLOAD_BYTES=int(os.environ.get('MAX_UPLOAD_BYTES', 20 * 1024 * 1024)),
        DATABASE=str(Path(os.environ.get('DATABASE', 'data/app.db')).resolve()),
        UPLOAD_ROOT=str(Path(os.environ.get('UPLOAD_ROOT', 'uploads')).resolve()),
    )
    if test_config:
        app.config.update(test_config)
    from app.tokens import validate_secret
    validate_secret(app.config['JWT_SIGNING_SECRET'])
    # No Session interface or CSRF hook participates in bearer authentication.
    app.config['SECRET_KEY'] = None
    from app.db import close_db
    app.teardown_appcontext(close_db)
    from app.auth import bp
    app.register_blueprint(bp)
    from app.routes import bp as materials_bp
    app.register_blueprint(materials_bp)
    from app.health import bp as health_bp
    app.register_blueprint(health_bp)
    from app.retrieval_setup import setup
    setup(app)
    from app.retrieval_routes import bp as retrieval_bp
    app.register_blueprint(retrieval_bp)
    @app.after_request
    def private_responses(response):
        from flask import request
        if request.path.startswith('/api/') or request.path in ('/login', '/materials', '/knowledge'):
            response.headers['Cache-Control'] = 'no-store'
        response.headers['X-Content-Type-Options'] = 'nosniff'
        response.headers['Referrer-Policy'] = 'same-origin'
        response.headers['Content-Security-Policy'] = (
            "default-src 'self'; script-src 'self'; connect-src 'self'; "
            "object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"
        )
        return response
    return app
