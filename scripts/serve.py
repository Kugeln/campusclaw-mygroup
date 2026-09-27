"""Validate configuration and recover storage before starting any workers."""
import os
from pathlib import Path
import sys
import subprocess
from contextlib import closing

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from app import create_app
from app.db import connect
from app.recovery import recover_storage
from scripts.init_db import prepare_sessions


def prepare():
    app = create_app()
    database = Path(app.config['DATABASE'])
    if not database.is_file():
        subprocess.run([sys.executable, str(Path(__file__).with_name('init_db.py')), '--fresh', '--database', str(database), '--uploads', app.config['UPLOAD_ROOT']], check=True)
    with closing(connect(database)) as db:
        prepare_sessions(db)
        recover_storage(db, app.config['UPLOAD_ROOT'])


if __name__ == '__main__':
    prepare()
    os.execvp('gunicorn', ['gunicorn', '--bind', '0.0.0.0:8080', '--workers', '1', 'wsgi:app'])
