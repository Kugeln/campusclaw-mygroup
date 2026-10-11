"""Run once before workers; never clean files while uploads are in progress."""
from pathlib import Path
from uuid import UUID


def recover_storage(db, upload_root, material_storage=None):
    root = Path(upload_root)
    staging, final = root / 'tmp', root / 'files'
    for directory in (root, staging, final):
        if directory.is_symlink():
            raise RuntimeError('Unsafe storage directory')
        directory.mkdir(parents=True, exist_ok=True)
    rows = (material_storage.recovery_rows() if material_storage else db.execute('SELECT m.storage_key,k.id AS text_id FROM materials m LEFT JOIN knowledge_entries k ON k.material_id=m.id AND k.class_id=m.class_id').fetchall())
    referenced = set()
    for row in rows:
        key = row['storage_key']
        try:
            if str(UUID(key)) != key:
                raise ValueError()
        except (ValueError, TypeError):
            raise RuntimeError('Invalid stored material key') from None
        if row['text_id'] is None or not (final / key).is_file() or (final / key).is_symlink():
            raise RuntimeError('Incomplete material; restore data before startup')
        referenced.add(key)
    for directory in (staging, final):
        for path in directory.iterdir():
            if path.is_symlink() or not path.is_file():
                raise RuntimeError('Unexpected storage entry')
            if directory == staging or path.name not in referenced:
                path.unlink()
