from contextlib import closing
from pathlib import Path
import secrets
import sqlite3
from tempfile import TemporaryFile

from flask import Blueprint, current_app, jsonify
from app.retrieval.types import Unavailable

bp = Blueprint('health', __name__)


@bp.get('/health')
def health():
    try:
        database = Path(current_app.config['DATABASE']).resolve()
        with closing(sqlite3.connect(database.as_uri() + '?mode=ro', uri=True)) as db:
            db.execute('SELECT id FROM classes LIMIT 1').fetchall()
        storage = current_app.extensions.get('material_storage')
        if storage:
            with storage.transaction() as cursor:
                cursor.execute('SELECT id FROM materials LIMIT 1')
        root = Path(current_app.config['UPLOAD_ROOT'])
        for directory in (root, root / 'tmp', root / 'files'):
            with TemporaryFile(dir=directory) as probe:
                value = secrets.token_bytes(16)
                probe.write(value)
                probe.flush()
                probe.seek(0)
                if probe.read() != value:
                    raise OSError('Storage probe failed')
        return jsonify(status='ok'), 200
    except (OSError, sqlite3.Error, Unavailable):
        return jsonify(status='unhealthy'), 503
