from dataclasses import dataclass, asdict


class InvalidInput(ValueError):
    pass


class Unavailable(RuntimeError):
    pass


class InvalidAnswer(RuntimeError):
    pass


class Busy(RuntimeError):
    pass


@dataclass(frozen=True)
class Scope:
    user_id: int
    role: str
    class_id: int

    def __post_init__(self):
        if type(self.class_id) is not int or self.class_id <= 0:
            raise InvalidInput('Invalid trusted scope')


@dataclass(frozen=True)
class Evidence:
    chunk_id: str
    material_id: str
    knowledge_entry_id: int
    title: str
    chunk_index: int
    chunk_text: str
    score: float = 0.0

    def public(self):
        result = asdict(self)
        result.pop('knowledge_entry_id')
        return result
