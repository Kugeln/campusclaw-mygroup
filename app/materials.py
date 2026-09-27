"""Shared text parsing and material persistence for seeds and uploads."""
from pathlib import Path
from uuid import uuid4
from contextlib import contextmanager
from io import BytesIO

from werkzeug.exceptions import BadRequest, RequestEntityTooLarge
from werkzeug.utils import secure_filename
from werkzeug.datastructures import FileStorage


@contextmanager
def prepare_upload(upload_root, file, max_bytes=20 * 1024 * 1024):
    if file is None or not file.filename or Path(file.filename).suffix.lower() not in ('.txt', '.md'):
        raise BadRequest('A .txt or .md file is required')
    staging = Path(upload_root) / 'tmp'
    staging.mkdir(parents=True, exist_ok=True)
    temporary = staging / str(uuid4())
    try:
        with temporary.open('xb') as target:
            size = 0
            while chunk := file.stream.read(65536):
                size += len(chunk)
                if size > max_bytes:
                    raise RequestEntityTooLarge('File exceeds upload limit')
                target.write(chunk)
        try:
            text = parse_text(temporary.read_bytes())
        except (ValueError, UnicodeError):
            raise BadRequest('Invalid UTF-8 text file') from None
        yield temporary, text
    finally:
        temporary.unlink(missing_ok=True)


def parse_text(content):
    text = content.decode('utf-8-sig')
    if not content or '\x00' in text:
        raise ValueError('Invalid text file')
    return text


def persist_material(db, upload_root, name, content, class_id, uploader_id):
    file = FileStorage(stream=BytesIO(content), filename=name)
    with prepare_upload(upload_root, file) as (temporary, text):
        return persist_prepared(db, upload_root, name, temporary, text, class_id, uploader_id)


def persist_prepared(db, upload_root, name, temporary, text, class_id, uploader_id):
    name = secure_filename(name) or 'material.txt'
    material_id = str(uuid4())
    root = Path(upload_root)
    final = root / 'files'
    final.mkdir(parents=True, exist_ok=True)
    destination = final / material_id
    try:
        with db:
            db.execute(
                'INSERT INTO materials(id,class_id,uploader_id,original_name,size,content_type,storage_key) VALUES (?,?,?,?,?,?,?)',
                (material_id, class_id, uploader_id, name, temporary.stat().st_size, 'text/plain', material_id),
            )
            db.execute('INSERT INTO knowledge_entries(material_id,class_id,plain_text) VALUES (?,?,?)', (material_id, class_id, text))
            temporary.replace(destination)
    except Exception:
        temporary.unlink(missing_ok=True)
        destination.unlink(missing_ok=True)
        raise
    return material_id


def seed_materials(db, upload_root):
    for class_name, username in (('A', 'teacher_a'), ('B', 'teacher_b')):
        user = db.execute('SELECT id,class_id FROM users WHERE username=?', (username,)).fetchone()
        name = f'Class-{class_name}-sample.txt'
        if not db.execute('SELECT id FROM materials WHERE class_id=? AND original_name=?', (user[1], name)).fetchone():
            persist_material(db, upload_root, name, f'Class {class_name} teaching material\n'.encode(), user[1], user[0])
