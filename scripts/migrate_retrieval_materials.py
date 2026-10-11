"""Explicit SQLite material-only import; never run automatically at startup."""
import argparse
import os
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from app.storage.mysql import MySQLStorage
from app.storage.migration import migrate_materials


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', required=True)
    parser.add_argument('--uploads', required=True)
    args = parser.parse_args()
    names = ('MYSQL_HOST', 'MYSQL_DATABASE', 'MYSQL_USER', 'MYSQL_PASSWORD')
    missing = [name for name in names if not os.environ.get(name)]
    if missing:
        raise SystemExit('Missing configuration: ' + ', '.join(missing))
    storage = MySQLStorage(dict(os.environ))
    storage.initialize()
    print(migrate_materials(args.source, args.uploads, storage))


if __name__ == '__main__':
    main()
