import pytest

from app.db import connect, find_material, list_materials
from app.materials import persist_material
from scripts.init_db import initialize


def test_scoped_queries(tmp_path):
    database = tmp_path / 'app.db'
    initialize(database)
    with connect(database) as db:
        assert db.execute('PRAGMA foreign_keys').fetchone()[0] == 1
        db.executemany('INSERT INTO classes VALUES (?,?)', [(1, 'A'), (2, 'B')])
        db.executemany('INSERT INTO users(id,username,password_hash,role,class_id) VALUES (?,?,?,?,?)', [(1, 'ta', 'hash', 'teacher', 1), (2, 'tb', 'hash', 'teacher', 2)])
    with connect(database) as db:
        a = persist_material(db, tmp_path / 'uploads', 'A.txt', b'A', 1, 1)
        b = persist_material(db, tmp_path / 'uploads', 'B.txt', b'B', 2, 2)
        assert [r['id'] for r in list_materials(db, 1)] == [a]
        assert [r['id'] for r in list_materials(db, 2)] == [b]
        assert find_material(db, a, 1)['plain_text'] == 'A'
        assert find_material(db, b, 1) is None
        assert find_material(db, "' OR 1=1 --", 1) is None
        for fn, args in [(list_materials, (db,)), (find_material, (db, a))]:
            with pytest.raises(TypeError):
                fn(*args)
            with pytest.raises(ValueError):
                fn(*args, None)
