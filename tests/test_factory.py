from flask import Flask

from app import create_app


def test_factory_does_not_initialize_database(tmp_path):
    database = tmp_path / 'app.db'
    app = create_app({'TESTING': True, 'DATABASE': str(database)})
    assert isinstance(app, Flask)
    assert app.testing
    assert not database.exists()
