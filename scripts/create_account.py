"""Provision one class or one user; passwords never appear in command arguments."""
import argparse
from getpass import getpass
import os
from pathlib import Path
import sqlite3
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from app.accounts import create_class, create_user


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--database', default=os.environ.get('DATABASE', 'data/app.db'))
    parser.add_argument('--class-name', required=True)
    parser.add_argument('--username')
    parser.add_argument('--role', choices=('teacher', 'student'))
    args = parser.parse_args()
    if args.username and not args.role:
        parser.error('--role is required with --username')
    with sqlite3.connect(args.database) as db:
        db.execute('PRAGMA foreign_keys = ON')
        class_id = create_class(db, args.class_name)
        if args.username:
            password = getpass('Password: ') if sys.stdin.isatty() else sys.stdin.readline().rstrip('\r\n')
            create_user(db, args.username, password, args.role, class_id)
    print('Provisioning complete.')


if __name__ == '__main__':
    main()
