import os


CONFIG_NAMES = ('MYSQL_HOST', 'MYSQL_PORT', 'MYSQL_DATABASE', 'MYSQL_USER', 'MYSQL_PASSWORD',
                'QDRANT_URL', 'EMBEDDING_BASE_URL', 'EMBEDDING_API_KEY', 'EMBEDDING_MODEL',
                'EMBEDDING_DIMENSION', 'DIALOGUE_BASE_URL', 'DIALOGUE_API_KEY', 'DIALOGUE_MODEL')


def setup(app):
    app.config.setdefault('MATERIAL_BACKEND', os.environ.get('MATERIAL_BACKEND', 'sqlite'))
    if app.config['MATERIAL_BACKEND'] == 'sqlite':
        return
    if app.config['MATERIAL_BACKEND'] != 'mysql':
        raise ValueError('Invalid MATERIAL_BACKEND')
    for name in CONFIG_NAMES:
        app.config.setdefault(name, os.environ.get(name))
    app.config['MYSQL_PORT'] = app.config['MYSQL_PORT'] or '3306'
    missing = [name for name in CONFIG_NAMES if not app.config.get(name)]
    if missing:
        raise ValueError('Missing required configuration: ' + ', '.join(missing))
    try:
        dimension = int(app.config['EMBEDDING_DIMENSION'])
        if dimension <= 0:
            raise ValueError()
    except (ValueError, TypeError):
        raise ValueError('Invalid EMBEDDING_DIMENSION') from None
    from app.storage.mysql import MySQLStorage
    from app.clients.embedding import Embedding
    from app.clients.dialogue import Dialogue
    from app.clients.qdrant import Qdrant
    from app.retrieval.service import Retrieval
    from app.retrieval.indexing import Indexing
    from app.retrieval.answering import Answering
    config = app.config
    config.setdefault('EMBEDDING_API_FORMAT', os.environ.get('EMBEDDING_API_FORMAT', 'ark_multimodal'))
    storage = MySQLStorage(config)
    embedding = Embedding(config['EMBEDDING_BASE_URL'], config['EMBEDDING_API_KEY'], config['EMBEDDING_MODEL'], dimension, config['EMBEDDING_API_FORMAT'])
    dialogue = Dialogue(config['DIALOGUE_BASE_URL'], config['DIALOGUE_API_KEY'], config['DIALOGUE_MODEL'])
    vectors = Qdrant(config['QDRANT_URL'], dimension)
    retrieval = Retrieval(storage, embedding, vectors)
    app.extensions.update(material_storage=storage, retrieval=retrieval,
                          indexing=Indexing(storage, embedding, vectors), answering=Answering(retrieval, dialogue))
