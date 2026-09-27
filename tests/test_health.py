from pathlib import Path
from unittest.mock import patch


def test_health_dependencies(app, client, tmp_path):
    response = client.get('/health')
    assert response.status_code == 200 and response.json == {'status': 'ok'}
    original = app.config['DATABASE']
    app.config['DATABASE'] = str(tmp_path / 'missing.db')
    response = client.get('/health')
    assert response.status_code == 503 and response.json == {'status': 'unhealthy'}
    assert not Path(app.config['DATABASE']).exists()
    app.config['DATABASE'] = original
    with patch('app.health.TemporaryFile', side_effect=OSError('private path or secret')):
        response = client.get('/health')
        assert response.status_code == 503 and response.json == {'status': 'unhealthy'}
    assert not list((Path(app.config['UPLOAD_ROOT']) / 'tmp').iterdir())
