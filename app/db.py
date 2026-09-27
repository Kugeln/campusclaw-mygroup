import sqlite3

from flask import current_app, g


def connect(database):
    db = sqlite3.connect(database)
    db.row_factory = sqlite3.Row
    db.execute('PRAGMA foreign_keys = ON')
    return db


def get_db():
    if 'db' not in g:
        g.db = connect(current_app.config['DATABASE'])
    return g.db


def close_db(error=None):
    db = g.pop('db', None)
    if db is not None:
        db.close()


def require_class(class_id):
    if not isinstance(class_id, int) or isinstance(class_id, bool) or class_id <= 0:
        raise ValueError('A valid class_id is required')


def list_materials(db, class_id):
    require_class(class_id)
    return db.execute(
        'SELECT m.id,m.original_name,m.size,m.created_at,u.username AS uploader '
        'FROM materials m JOIN users u ON u.id=m.uploader_id '
        'WHERE m.class_id=? ORDER BY m.created_at DESC,m.id DESC', (class_id,),
    ).fetchall()


def find_material(db, material_id, class_id):
    require_class(class_id)
    return db.execute(
        'SELECT m.*,u.username AS uploader,k.plain_text FROM materials m '
        'JOIN users u ON u.id=m.uploader_id '
        'JOIN knowledge_entries k ON k.material_id=m.id AND k.class_id=m.class_id '
        'WHERE m.id=? AND m.class_id=?', (material_id, class_id),
    ).fetchone()
