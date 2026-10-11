from pathlib import Path
from unittest.mock import patch
import secrets

import pytest

from app import create_app
from app.retrieval_setup import CONFIG_NAMES


def test_missing_configuration_names_only(monkeypatch):
    for name in CONFIG_NAMES:
        monkeypatch.delenv(name, raising=False)
    secret = secrets.token_urlsafe(32)
    with pytest.raises(ValueError) as error:
        create_app(dict(MATERIAL_BACKEND='mysql', EMBEDDING_API_KEY=secret))
    message = str(error.value)
    assert 'MYSQL_HOST' in message and 'DIALOGUE_API_KEY' in message
    assert secret not in message


def test_qdrant_is_not_a_factory_dependency(monkeypatch):
    config = {name:'test' for name in CONFIG_NAMES}
    config.update(MATERIAL_BACKEND='mysql', MYSQL_PORT=3306, EMBEDDING_DIMENSION=2048)
    with patch('app.clients.http.urlopen', side_effect=AssertionError('No external calls during factory')):
        app = create_app(config)
    assert app.extensions['indexing'].vectors.dimension == 2048
    assert app.extensions['indexing'].embedding.api_format == 'ark_multimodal'
    with pytest.raises(ValueError, match='EMBEDDING_API_FORMAT'):
        create_app(dict(config, EMBEDDING_API_FORMAT='unknown'))


def test_classroom_compose_keeps_database_ports_private():
    text = (Path(__file__).resolve().parents[2] / 'docker-compose.retrieval.yml').read_text(encoding='utf-8')
    services = text.split('\n  mysql:\n', 1)[1]
    assert 'ports:' not in services
    assert 'mysql:8.4.4' in text and 'qdrant/qdrant:v1.13.4' in text
    app = text.split('\n  mysql:\n', 1)[0]
    assert 'condition: service_healthy' in app
    assert 'qdrant:\n' not in app
    assert 'nginx' not in text and 'redis' not in text
