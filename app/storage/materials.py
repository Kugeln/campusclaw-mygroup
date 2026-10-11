"""One selected authority for material routes; identity stays in SQLite."""
from flask import current_app

from app.db import get_db, find_material, list_materials
from app.materials import persist_prepared


def selected():
    return current_app.extensions.get('material_storage')


def uploader_names(rows):
    ids = list({row['uploader_id'] for row in rows})
    if not ids:
        return rows
    users = get_db().execute('SELECT id,username FROM users WHERE id IN (' + ','.join('?' for _ in ids) + ')', ids).fetchall()
    names = {user['id']: user['username'] for user in users}
    for row in rows:
        row['uploader'] = names.get(row['uploader_id'], '')
    return rows


def listing(class_id):
    storage = selected()
    return uploader_names(storage.list_materials(class_id)) if storage else list_materials(get_db(), class_id)


def find(material_id, class_id):
    storage = selected()
    if not storage:
        return find_material(get_db(), material_id, class_id)
    row = storage.find_material(material_id, class_id)
    return uploader_names([row])[0] if row else None


def persist(root, name, temporary, text, class_id, user_id):
    storage = selected()
    if storage:
        return storage.persist(root, name, temporary, text, class_id, user_id)
    return persist_prepared(get_db(), root, name, temporary, text, class_id, user_id)
