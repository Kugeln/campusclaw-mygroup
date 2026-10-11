from contextlib import contextmanager
from pathlib import Path
from uuid import uuid4
import json

import pymysql
from werkzeug.utils import secure_filename

from app.retrieval.types import Evidence, Unavailable


class MySQLStorage:
    def __init__(self, config):
        self.config = dict(host=config['MYSQL_HOST'], port=int(config.get('MYSQL_PORT', 3306)),
                           user=config['MYSQL_USER'], password=config['MYSQL_PASSWORD'],
                           database=config['MYSQL_DATABASE'], charset='utf8mb4',
                           cursorclass=pymysql.cursors.DictCursor, connect_timeout=5,
                           read_timeout=15, write_timeout=15, init_command="SET time_zone = '+00:00'")

    @contextmanager
    def transaction(self):
        db = None
        try:
            db = pymysql.connect(**self.config)
            with db.cursor() as cursor:
                yield cursor
            db.commit()
        except pymysql.MySQLError:
            if db:
                db.rollback()
            raise Unavailable('Material storage unavailable') from None
        except Exception:
            if db:
                db.rollback()
            raise
        finally:
            if db:
                db.close()

    def initialize(self):
        with self.transaction() as cur:
            cur.execute('SELECT @@ngram_token_size AS size')
            if cur.fetchone()['size'] != 2:
                raise Unavailable('MySQL requires ngram_token_size=2')
            for statement in Path(__file__).with_name('retrieval.sql').read_text(encoding='utf-8').split(';'):
                if statement.strip():
                    cur.execute(statement)
            cur.execute('SHOW CREATE TABLE knowledge_chunks')
            if 'ngram' not in cur.fetchone()['Create Table']:
                raise Unavailable('MySQL requires FULLTEXT ngram')

    def list_materials(self, class_id):
        with self.transaction() as cur:
            cur.execute('SELECT m.*,k.index_status FROM materials m JOIN knowledge_entries k '
                        'ON k.material_id=m.id AND k.class_id=m.class_id '
                        'WHERE m.class_id=%s ORDER BY m.created_at DESC,m.id DESC', (class_id,))
            return cur.fetchall()

    def find_material(self, material_id, class_id):
        with self.transaction() as cur:
            cur.execute('SELECT m.*,k.id AS knowledge_entry_id,k.body_text AS plain_text,'
                        'k.index_status,k.cleanup_required FROM materials m JOIN knowledge_entries k '
                        'ON k.material_id=m.id AND k.class_id=m.class_id WHERE m.id=%s AND m.class_id=%s',
                        (material_id, class_id))
            return cur.fetchone()

    def persist(self, root, name, temporary, text, class_id, uploader_id):
        material_id = str(uuid4())
        destination = Path(root) / 'files' / material_id
        destination.parent.mkdir(parents=True, exist_ok=True)
        try:
            with self.transaction() as cur:
                cur.execute('INSERT INTO materials(id,class_id,uploader_id,original_name,size,content_type,storage_key) '
                            'VALUES (%s,%s,%s,%s,%s,%s,%s)',
                            (material_id, class_id, uploader_id, secure_filename(name) or 'material.txt',
                             temporary.stat().st_size, 'text/plain', material_id))
                cur.execute('INSERT INTO knowledge_entries(material_id,class_id,body_text) VALUES (%s,%s,%s)',
                            (material_id, class_id, text))
                temporary.replace(destination)
        except Exception:
            destination.unlink(missing_ok=True)
            raise
        return material_id

    def recovery_rows(self):
        with self.transaction() as cur:
            cur.execute('SELECT m.storage_key,k.id AS text_id FROM materials m LEFT JOIN knowledge_entries k '
                        'ON k.material_id=m.id AND k.class_id=m.class_id')
            return cur.fetchall()

    def seed_materials(self, identity_db, upload_root):
        from io import BytesIO
        from werkzeug.datastructures import FileStorage
        from app.materials import prepare_upload
        for group, username in (('A', 'teacher_a'), ('B', 'teacher_b')):
            user = identity_db.execute('SELECT id,class_id,role FROM users WHERE username=?', (username,)).fetchone()
            if user is None or user['role'] != 'teacher':
                raise ValueError('Sample teacher missing')
            name = f'Class-{group}-sample.txt'
            if any(row['original_name'] == name for row in self.list_materials(user['class_id'])):
                continue
            file = FileStorage(stream=BytesIO(f'Class {group} teaching material\n'.encode()), filename=name)
            with prepare_upload(upload_root, file) as (temporary, text):
                self.persist(upload_root, name, temporary, text, user['class_id'], user['id'])

    def interrupted(self):
        with self.transaction() as cur:
            cur.execute("SELECT material_id,class_id FROM knowledge_entries WHERE index_status IN ('pending','indexing') OR cleanup_required=1")
            return cur.fetchall()

    def state(self, class_id, material_id, state, cleanup=False):
        with self.transaction() as cur:
            cur.execute('UPDATE knowledge_entries SET index_status=%s,cleanup_required=%s WHERE class_id=%s AND material_id=%s',
                        (state, cleanup, class_id, material_id))
            cur.execute('UPDATE knowledge_chunks SET index_status=%s WHERE class_id=%s AND material_id=%s',
                        ('pending' if state == 'indexing' else state, class_id, material_id))

    def replace_chunks(self, material, texts, strategy):
        pieces = [dict(chunk_id=str(uuid4()), knowledge_entry_id=material['knowledge_entry_id'],
                       material_id=material['id'], class_id=material['class_id'], chunk_index=i, chunk_text=text)
                  for i, text in enumerate(texts, 1)]
        with self.transaction() as cur:
            cur.execute('DELETE FROM knowledge_chunks WHERE material_id=%s AND class_id=%s', (material['id'], material['class_id']))
            cur.execute('UPDATE knowledge_entries SET chunk_strategy=%s WHERE material_id=%s AND class_id=%s',
                        (json.dumps(strategy), material['id'], material['class_id']))
            for p in pieces:
                cur.execute('INSERT INTO knowledge_chunks(id,knowledge_entry_id,material_id,class_id,chunk_index,chunk_text) '
                            'VALUES (%s,%s,%s,%s,%s,%s)', tuple(p[k] for k in
                            ('chunk_id', 'knowledge_entry_id', 'material_id', 'class_id', 'chunk_index', 'chunk_text')))
        return pieces

    evidence_sql = ("SELECT c.id AS chunk_id,c.material_id,c.knowledge_entry_id,m.original_name AS title,"
                    "c.chunk_index,c.chunk_text FROM knowledge_chunks c JOIN knowledge_entries k "
                    "ON k.id=c.knowledge_entry_id AND k.material_id=c.material_id AND k.class_id=c.class_id "
                    "JOIN materials m ON m.id=c.material_id AND m.class_id=c.class_id "
                    "WHERE c.class_id=%s AND c.index_status='ready' AND k.index_status='ready'")

    def hydrate(self, class_id, ids):
        if not ids:
            return {}
        with self.transaction() as cur:
            cur.execute(self.evidence_sql + ' AND c.id IN (' + ','.join(['%s'] * len(ids)) + ')', (class_id, *ids))
            return {row['chunk_id']: Evidence(**row) for row in cur.fetchall()}

    def keyword(self, class_id, query, limit):
        match = 'MATCH(c.chunk_text) AGAINST (%s IN NATURAL LANGUAGE MODE)'
        sql = self.evidence_sql.replace(' FROM knowledge_chunks', ', ' + match + ' AS score FROM knowledge_chunks')
        with self.transaction() as cur:
            cur.execute(sql + ' AND ' + match + '>0 ORDER BY score DESC,c.id ASC LIMIT %s',
                        (query, class_id, query, limit))
            return [Evidence(**row) for row in cur.fetchall() if row['chunk_text'].strip()]
