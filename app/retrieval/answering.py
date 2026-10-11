import re

from .types import InvalidAnswer

NO_EVIDENCE = '资料中未找到相关内容'


class Answering:
    def __init__(self, retrieval, dialogue):
        self.retrieval, self.dialogue = retrieval, dialogue

    def ask(self, scope, question, mode='hybrid'):
        evidence = self.retrieval.search(scope, question, mode, limit=4)
        if not evidence:
            return dict(answer=NO_EVIDENCE, sources=[])
        numbered = [dict(number=i, title=e.title, chunk_index=e.chunk_index, chunk_text=e.chunk_text)
                    for i, e in enumerate(evidence, 1)]
        answer = self.dialogue.answer(question.strip(), numbered)
        if not isinstance(answer, str) or not answer.strip():
            raise InvalidAnswer('Invalid answer')
        numbers = set(int(n) for n in re.findall(r'\[(\d+)\]', answer))
        if not numbers or not numbers.issubset(range(1, len(evidence) + 1)):
            raise InvalidAnswer('Invalid citations')
        sources = []
        for number in sorted(numbers):
            source = evidence[number - 1].public()
            source.pop('score')
            sources.append(dict(number=number, **source))
        return dict(answer=answer.strip(), sources=sources)
