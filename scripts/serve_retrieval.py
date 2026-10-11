"""Separate small-demo startup. No migration and no reads of legacy material snapshots."""
import os
from pathlib import Path
import sys
from contextlib import closing
from tempfile import TemporaryDirectory

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from app import create_app
from app.accounts import seed_accounts
from app.db import connect
from app.recovery import recover_storage
from scripts.init_db import initialize, environment_password


def prepare():
    if os.environ.get('MATERIAL_BACKEND') != 'mysql':
        raise ValueError('Demo requires MATERIAL_BACKEND=mysql')
    app = create_app()
    database = Path(app.config['DATABASE'])
    fresh = not database.exists()
    if fresh:
        database.parent.mkdir(parents=True, exist_ok=True)
        with TemporaryDirectory(prefix='.identity-', dir=database.parent) as staging:
            pending = Path(staging) / 'identity.db'
            initialize(pending)
            with closing(connect(pending)) as db:
                with db:
                    seed_accounts(db, environment_password)
            pending.replace(database)
    storage = app.extensions['material_storage']
    storage.initialize()
    with closing(connect(database)) as db:
        if fresh:
            storage.seed_materials(db, app.config['UPLOAD_ROOT'])
        recover_storage(db, app.config['UPLOAD_ROOT'], storage)
    app.extensions['indexing'].recover()
    return app


if __name__ == '__main__':
    app = prepare()
    if os.name == 'nt':
        app.run(host='127.0.0.1', port=8080, debug=False, use_reloader=False)
    else:
        os.execvp('gunicorn', ['gunicorn', '--bind', '0.0.0.0:8080', '--workers', '1', '--timeout', '180', 'wsgi:app'])
