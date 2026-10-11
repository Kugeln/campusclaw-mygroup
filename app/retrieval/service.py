from dataclasses import replace
import math

from .ranking import rrf
from .types import InvalidInput


class Retrieval:
    def __init__(self, storage, embedding, vectors):
        self.storage, self.embedding, self.vectors = storage, embedding, vectors

    def search(self, scope, query, mode='hybrid', limit=4):
        if not isinstance(query, str) or not query.strip() or mode not in ('keyword', 'vector', 'hybrid'):
            raise InvalidInput('A question and valid mode are required')
        query = query.strip()
        keyword, vector = [], []
        if mode in ('keyword', 'hybrid'):
            keyword = self.storage.keyword(scope.class_id, query, 20)
        if mode in ('vector', 'hybrid'):
            hits = self.vectors.search(scope.class_id, self.embedding.embed(query), 20)
            resolved = self.storage.hydrate(scope.class_id, [hit['id'] for hit in hits])
            for hit in hits:
                entry = resolved.get(hit['id'])
                score, payload = hit.get('score', 0), hit.get('payload', {})
                if not entry or not entry.chunk_text.strip() or not isinstance(score, (float, int)) or not math.isfinite(score) or score < .35:
                    continue
                expected = dict(class_id=scope.class_id, material_id=entry.material_id,
                                knowledge_entry_id=entry.knowledge_entry_id, chunk_id=entry.chunk_id,
                                chunk_index=entry.chunk_index)
                if any(payload.get(key) != value for key, value in expected.items()):
                    continue
                vector.append(replace(entry, score=score))
        def ordered(entries):
            result = {}
            for entry in sorted(entries, key=lambda e: (-e.score, e.chunk_id)):
                if entry.chunk_text.strip() and entry.score > 0:
                    result.setdefault(entry.chunk_id, entry)
            return list(result.values())
        keyword, vector = ordered(keyword), ordered(vector)
        if mode != 'hybrid':
            return (keyword if mode == 'keyword' else vector)[:limit]
        evidence = {e.chunk_id: e for e in keyword + vector}
        return [replace(evidence[key], score=score) for key, score in
                rrf([e.chunk_id for e in keyword], [e.chunk_id for e in vector])[:limit]]
