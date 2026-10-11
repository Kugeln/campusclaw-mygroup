from flask import Blueprint, current_app, g, jsonify, render_template, request

from app.auth import authenticated, teacher_required
from app.retrieval.types import Scope, InvalidInput, Unavailable, InvalidAnswer, Busy

bp = Blueprint('retrieval', __name__)


def scope():
    p = g.principal
    return Scope(p.user_id, p.role, p.class_id)


def service(name):
    result = current_app.extensions.get(name)
    if result is None:
        raise Unavailable('Retrieval requires MySQL configuration')
    return result


@bp.app_errorhandler(Unavailable)
def unavailable(error):
    return jsonify(error='Retrieval dependency unavailable'), 503


@bp.app_errorhandler(InvalidInput)
def invalid(error):
    return jsonify(error=str(error)), 400


@bp.app_errorhandler(InvalidAnswer)
def invalid_answer(error):
    return jsonify(error='Answer or citations unavailable'), 502


@bp.app_errorhandler(Busy)
def busy(error):
    return jsonify(error='Material indexing in progress'), 409


@bp.get('/knowledge')
def knowledge_page():
    return render_template('knowledge.html')


@bp.get('/api/search')
@authenticated()
def search():
    mode = request.args.get('mode', 'hybrid')
    items = service('retrieval').search(scope(), request.args.get('q'), mode)
    return jsonify(mode=mode, items=[item.public() for item in items])


@bp.post('/api/ask')
@authenticated()
def ask():
    data = request.get_json(silent=True)
    if not isinstance(data, dict):
        raise InvalidInput('JSON object required')
    return jsonify(service('answering').ask(scope(), data.get('question'), data.get('mode', 'hybrid')))


@bp.post('/api/materials/<material_id>/reindex')
@authenticated()
@teacher_required
def reindex(material_id):
    data = request.get_json(silent=True)
    if not isinstance(data, dict):
        raise InvalidInput('JSON object required')
    try:
        status = service('indexing').run(scope(), material_id, data)
    except LookupError:
        return jsonify(error='Material not found'), 404
    return jsonify(material_id=material_id, index_status=status)
