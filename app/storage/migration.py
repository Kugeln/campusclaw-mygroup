"""Explicit material-only transfer. Source SQLite and original files stay read-only."""
from contextlib import closing
from datetime import datetime, timezone
from pathlib import Path
from uuid import UUID
import hashlib
import sqlite3

from app.materials import parse_text


def timestamp(value):
    result = datetime.fromisoformat(value.replace('Z', '+00:00')) if isinstance(value, str) else value
    if result.tzinfo:
        result = result.astimezone(timezone.utc).replace(tzinfo=None)
    return result


def migrate_materials(source, upload_root, storage):
    source = Path(source).resolve()
    files = (Path(upload_root).resolve() / 'files')
    with closing(sqlite3.connect(source.as_uri() + '?mode=ro', uri=True)) as db:
        db.row_factory = sqlite3.Row
        db.execute('BEGIN')
        rows = db.execute('SELECT m.*,k.id AS entry_id,k.plain_text,k.created_at AS entry_created,'
                          'k.class_id AS entry_class,u.class_id AS user_class,c.id AS known_class '
                          'FROM materials m LEFT JOIN knowledge_entries k ON k.material_id=m.id '
                          'LEFT JOIN users u ON u.id=m.uploader_id LEFT JOIN classes c ON c.id=m.class_id '
                          'ORDER BY m.id').fetchall()
        if db.execute('SELECT COUNT(*) FROM knowledge_entries').fetchone()[0] != len(rows):
            raise ValueError('Incomplete source material relationships')
        prepared = []
        for row in rows:
            if (row['entry_id'] is None or row['known_class'] is None
                    or row['class_id'] != row['entry_class'] or row['class_id'] != row['user_class']):
                raise ValueError('Invalid source material relationships')
            key = row['storage_key']
            if str(UUID(key)) != key or str(UUID(row['id'])) != row['id']:
                raise ValueError('Invalid source material identifier')
            path = files / key
            if path.is_symlink() or not path.is_file() or path.resolve().parent != files:
                raise ValueError('Invalid source material file')
            content = path.read_bytes()
            if len(content) != row['size'] or parse_text(content) != row['plain_text']:
                raise ValueError('Source file and text differ')
            material = {key: row[key] for key in ('id', 'class_id', 'uploader_id', 'original_name', 'size', 'content_type', 'storage_key')}
            material['created_at'] = timestamp(row['created_at'])
            entry = dict(id=row['entry_id'], material_id=row['id'], class_id=row['class_id'],
                         body_text=row['plain_text'], created_at=timestamp(row['entry_created']))
            prepared.append((material, entry))
        inserted = 0
        with storage.transaction() as cur:
            for material, entry in prepared:
                cur.execute('SELECT * FROM materials WHERE id=%s OR storage_key=%s', (material['id'], material['storage_key']))
                old_material = cur.fetchall()
                cur.execute('SELECT * FROM knowledge_entries WHERE id=%s OR material_id=%s', (entry['id'], material['id']))
                old_entry = cur.fetchall()
                if old_material or old_entry:
                    if (len(old_material) != 1 or len(old_entry) != 1
                            or any(old_material[0][k] != v for k, v in material.items())
                            or any(old_entry[0][k] != v for k, v in entry.items())):
                        raise ValueError('Conflicting target material; no overwrite performed')
                    continue
                for table, record in (('materials', material), ('knowledge_entries', entry)):
                    cur.execute('INSERT INTO ' + table + '(' + ','.join(record) + ') VALUES (' +
                                ','.join(['%s'] * len(record)) + ')', tuple(record.values()))
                # Verify server timestamp precision and text before committing.
                cur.execute('SELECT body_text,created_at FROM knowledge_entries WHERE id=%s', (entry['id'],))
                saved = cur.fetchone()
                if saved['created_at'] != entry['created_at'] or hashlib.sha256(saved['body_text'].encode()).digest() != hashlib.sha256(entry['body_text'].encode()).digest():
                    raise ValueError('Target cannot preserve source precision or text')
                cur.execute('SELECT created_at FROM materials WHERE id=%s', (material['id'],))
                if cur.fetchone()['created_at'] != material['created_at']:
                    raise ValueError('Target cannot preserve source timestamp precision')
                inserted += 1
        return dict(materials=len(prepared), inserted=inserted, unchanged=len(prepared) - inserted)
