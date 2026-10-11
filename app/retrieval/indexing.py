from contextlib import contextmanager
from threading import Lock
from time import monotonic

from .chunking import chunks, validate_strategy
from .types import Busy, Unavailable


class Indexing:
    def __init__(self, storage, embedding, vectors, deadline=120):
        self.storage, self.embedding, self.vectors = storage, embedding, vectors
        self.deadline, self.active, self.guard = deadline, set(), Lock()

    @contextmanager
    def lock(self, material_id):
        with self.guard:
            if material_id in self.active:
                raise Busy('Material indexing in progress')
            self.active.add(material_id)
        try:
            yield
        finally:
            with self.guard:
                self.active.remove(material_id)

    def run(self, scope, material_id, options=None, fresh=False):
        strategy = validate_strategy(options)
        with self.lock(material_id):
            material = self.storage.find_material(material_id, scope.class_id)
            if material is None:
                raise LookupError('Material not found')
            started, wrote, cleanup = monotonic(), False, False
            # Reject incompatible configuration before hiding or deleting an old index.
            # Qdrant failure on a fresh upload is handled below after committing text.
            if not fresh:
                self.vectors.check_configuration()
            try:
                self.storage.state(scope.class_id, material_id, 'indexing', not fresh)
                if not fresh:
                    cleanup = True
                    self.vectors.ensure()
                    self.vectors.delete(scope.class_id, material_id)
                    cleanup = False
                pieces = self.storage.replace_chunks(material, chunks(material['plain_text'], strategy), strategy)
                vectors = []
                for piece in pieces:
                    if monotonic() - started >= self.deadline:
                        raise Unavailable('Indexing timed out')
                    vectors.append(self.embedding.embed(piece['chunk_text']))
                if monotonic() - started >= self.deadline:
                    raise Unavailable('Indexing timed out')
                if pieces:
                    self.vectors.ensure()
                    wrote = True
                    self.vectors.upsert(scope.class_id, pieces, vectors)
                self.storage.state(scope.class_id, material_id, 'ready')
                return 'ready'
            except Exception:
                if wrote:
                    try:
                        self.vectors.delete(scope.class_id, material_id)
                    except Exception:
                        cleanup = True
                self.storage.state(scope.class_id, material_id, 'failed', cleanup)
                raise Unavailable('Indexing failed; original material retained') from None

    def recover(self):
        for material in self.storage.interrupted():
            class_id, material_id = material['class_id'], material['material_id']
            try:
                self.vectors.check_configuration()
            except Exception:
                continue
            self.storage.state(class_id, material_id, 'failed', True)
            try:
                self.vectors.ensure()
                self.vectors.delete(class_id, material_id)
            except Exception:
                continue
            self.storage.state(class_id, material_id, 'failed', False)
