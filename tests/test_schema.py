from pathlib import Path
import sqlite3
import subprocess
import sys

import pytest


def test_schema_and_constraints(tmp_path):
    script = Path(__file__).resolve().parents[1] / 'scripts/init_db.py'
    for _ in range(2):
        subprocess.run([sys.executable, str(script)], cwd=tmp_path, check=True)
    database = tmp_path / 'data/app.db'
    assert database.exists()
    with sqlite3.connect(database) as db:
        db.execute('PRAGMA foreign_keys = ON')
        expected = {
            'classes': {'id', 'name'},
            'users': {'id', 'username', 'password_hash', 'role', 'class_id', 'session_version'},
            'materials': {'id', 'class_id', 'uploader_id', 'original_name', 'size', 'content_type', 'storage_key', 'created_at'},
            'knowledge_entries': {'id', 'material_id', 'class_id', 'plain_text', 'created_at'},
            'sessions': {'sid_hash', 'data', 'expires_at'},
        }
        for table, columns in expected.items():
            assert {r[1] for r in db.execute(f'PRAGMA table_info({table})')} == columns
        db.executemany('INSERT INTO classes VALUES (?, ?)', [(1, 'A'), (2, 'B')])
        sql = 'INSERT INTO users(username,password_hash,role,class_id) VALUES (?,?,?,?)'
        db.execute(sql, ('teacher_a', 'hash', 'teacher', 1))
        for values in [('teacher_a', 'hash', 'teacher', 1), ('bad', 'hash', 'admin', 1), ('bad', 'hash', 'teacher', 999), ('bad', 'hash', 'teacher', None)]:
            with pytest.raises(sqlite3.IntegrityError):
                db.execute(sql, values)
        db.execute('INSERT INTO materials(id,class_id,uploader_id,original_name,size,content_type,storage_key) VALUES (?,?,?,?,?,?,?)', ('m1', 1, 1, 'A.txt', 1, 'text/plain', 'key'))
        with pytest.raises(sqlite3.IntegrityError):
            db.execute('INSERT INTO knowledge_entries(material_id,class_id,plain_text) VALUES (?,?,?)', ('m1', 2, 'cross class'))
        db.execute('INSERT INTO knowledge_entries(material_id,class_id,plain_text) VALUES (?,?,?)', ('m1', 1, 'same class'))


def test_core_structures_have_no_placeholder_business(tmp_path):
    from scripts.init_db import initialize
    from app import create_app

    database = tmp_path / 'app.db'
    initialize(database)
    with sqlite3.connect(database) as db:
        tables = {r[0] for r in db.execute("SELECT name FROM sqlite_master WHERE type='table'")}
        assert tables == {'classes', 'users', 'materials', 'knowledge_entries', 'assignments', 'assistants', 'skills', 'sessions'}
        for table in ('assignments', 'assistants', 'skills', 'materials'):
            assert db.execute(f'SELECT count(*) FROM {table}').fetchone()[0] == 0
    assert not any(any(word in rule.rule for word in ('assignments', 'assistants', 'skills')) for rule in create_app({'TESTING': True}).url_map.iter_rules())
