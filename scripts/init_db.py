"""Explicit, idempotent database initialization."""

import argparse
import os
from pathlib import Path
import sqlite3
import sys
from getpass import getpass
from contextlib import closing
from tempfile import TemporaryDirectory
import time

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from app.accounts import seed_accounts
from app.materials import seed_materials
from app.db import connect
from app.recovery import recover_storage


def environment_password(variable):
    password = os.environ.get(variable)
    if not password:
        raise ValueError(f'Missing required configuration: {variable}')
    return password


def initialize_fresh(database, uploads):
    """Publish the database only after all seeds and files are complete."""
    database, uploads = Path(database), Path(uploads)
    if database.exists():
        return
    database.parent.mkdir(parents=True, exist_ok=True)
    uploads.mkdir(parents=True, exist_ok=True)
    moved = []
    with TemporaryDirectory(prefix='.initialize-', dir=database.parent) as db_stage, TemporaryDirectory(prefix='.initialize-', dir=uploads) as file_stage:
        pending = Path(db_stage) / 'app.db'
        initialize(pending)
        try:
            with closing(connect(pending)) as db:
                with db:
                    seed_accounts(db, environment_password)
                seed_materials(db, file_stage)
                recover_storage(db, file_stage)
            final = uploads / 'files'
            final.mkdir(parents=True, exist_ok=True)
            (uploads / 'tmp').mkdir(parents=True, exist_ok=True)
            for file in (Path(file_stage) / 'files').iterdir():
                destination = final / file.name
                file.replace(destination)
                moved.append(destination)
            pending.replace(database)
        except Exception:
            for file in moved:
                file.unlink(missing_ok=True)
            raise


def initialize(database):
    database = Path(database)
    database.parent.mkdir(parents=True, exist_ok=True)
    with closing(sqlite3.connect(database)) as connection:
        connection.execute('PRAGMA foreign_keys = ON')
        connection.executescript(
            (Path(__file__).resolve().parents[1] / 'app/schema.sql').read_text(encoding='utf-8')
        )
        prepare_sessions(connection)


def prepare_sessions(connection):
    """Add session storage to new or existing databases without reseeding."""
    with connection:
        connection.execute(
            (Path(__file__).resolve().parents[1] / 'app/session_schema.sql').read_text(encoding='utf-8')
        )
        connection.execute('DELETE FROM sessions WHERE expires_at <= ?', (int(time.time()),))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--database', default=os.environ.get('DATABASE', 'data/app.db'))
    parser.add_argument('--seed', action='store_true')
    parser.add_argument('--uploads', default=os.environ.get('UPLOAD_ROOT', 'uploads'))
    parser.add_argument('--fresh', action='store_true', help='Initialize missing database and seeds atomically; preserve existing database')
    args = parser.parse_args()
    if os.environ.get('MATERIAL_BACKEND') == 'mysql':
        if args.fresh:
            from scripts.serve_retrieval import prepare
            prepare()
        elif args.seed:
            from app.storage.mysql import MySQLStorage
            initialize(args.database)
            storage = MySQLStorage(dict(os.environ))
            storage.initialize()
            with closing(connect(args.database)) as connection:
                with connection:
                    seed_accounts(connection, environment_password)
                storage.seed_materials(connection, args.uploads)
        else:
            raise SystemExit('Use --seed or --fresh for the MySQL material repository')
        print('MySQL material repository ready.')
        sys.exit(0)
    if args.fresh:
        initialize_fresh(args.database, args.uploads)
        print('Database ready.')
        sys.exit(0)
    initialize(args.database)
    if args.seed:
        def password_provider(variable):
            password = os.environ.get(variable)
            if not password and sys.stdin.isatty():
                password = getpass(f'{variable}: ')
            if not password:
                raise ValueError(f'Missing required configuration: {variable}')
            return password
        with sqlite3.connect(args.database) as connection:
            connection.execute('PRAGMA foreign_keys = ON')
            seed_accounts(connection, password_provider)
        with sqlite3.connect(args.database) as connection:
            connection.execute('PRAGMA foreign_keys = ON')
            seed_materials(connection, args.uploads)
    print('Database initialized.')
